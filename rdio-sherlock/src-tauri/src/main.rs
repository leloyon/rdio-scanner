//! Rdio Sherlock (shareable "Anywhere" build): a generic rdio-scanner
//! wrapper. Starts on a bundled setup page where the user enters any
//! rdio-scanner server address and an optional access code; the choice is
//! remembered on that machine. Frameless main window with a drag strip +
//! minimize/close buttons injected into the top of the page itself (close
//! tucks into the tray). Single instance: launching again just shows the
//! existing window.

#![cfg_attr(not(debug_assertions), windows_subsystem = "windows")]

use std::process::Command;
use std::sync::atomic::{AtomicBool, Ordering};
use std::sync::Mutex;
use std::time::Instant;
use tauri::{
    menu::{CheckMenuItem, Menu, MenuItem},
    tray::{MouseButton, TrayIconBuilder, TrayIconEvent},
    AppHandle, Manager, Runtime, WebviewUrl, WebviewWindowBuilder, Window, WindowEvent,
};

static MUTED: AtomicBool = AtomicBool::new(false);

/// Mute every audio/video element now, and keep muting new ones (rdio-scanner
/// creates elements per call) until unmuted.
const MUTE_JS: &str = r##"(function(){window.__rdioMuted=true;function m(n){if(n.tagName==='AUDIO'||n.tagName==='VIDEO')n.muted=true;if(n.querySelectorAll)n.querySelectorAll('audio,video').forEach(function(e){e.muted=true;});}m(document);if(!window.__rdioMuteObs){window.__rdioMuteObs=new MutationObserver(function(muts){if(!window.__rdioMuted)return;muts.forEach(function(mu){mu.addedNodes.forEach(function(n){if(n.nodeType===1)m(n);});});});window.__rdioMuteObs.observe(document.documentElement,{childList:true,subtree:true});}})();"##;
const UNMUTE_JS: &str = r##"(function(){window.__rdioMuted=false;document.querySelectorAll('audio,video').forEach(function(e){e.muted=false;});})();"##;

/// Runs before the page's own scripts on every load. The setup page hands
/// the access code over in the URL fragment (#rdioPin=<base64>); the
/// rdio-scanner client (v6.6.x and v8 alike) keeps its unlock code in
/// localStorage under 'rdio-scanner-pin' as base64, so seed (or clear) it
/// here, then scrub the fragment so it never lingers in the address.
const PIN_SEED_SCRIPT: &str = r##"(function () {
  try {
    var hash = window.location.hash || '';
    if (hash.indexOf('rdioPin=') === -1) return;
    var parts = hash.replace(/^#/, '').split('&');
    var value = null;
    for (var i = 0; i < parts.length; i++) {
      var kv = parts[i].split('=');
      if (kv[0] === 'rdioPin') {
        value = kv.slice(1).join('=');
        try { value = decodeURIComponent(value); } catch (e) {}
        break;
      }
    }
    if (value === null) return;
    try {
      if (value) {
        window.localStorage.setItem('rdio-scanner-pin', value);
      } else {
        window.localStorage.removeItem('rdio-scanner-pin');
      }
    } catch (e) {}
    try {
      window.history.replaceState(null, '', window.location.pathname + window.location.search);
    } catch (e) {}
  } catch (e) {}
})();"##;

/// Injected into the main window's page: a 28px strip across the top (the
/// drag handle, via data-tauri-drag-region; minimize/close buttons at the
/// right).
const INIT_SCRIPT: &str = r##"(function () {
  if (window.__rdioStrip) return;
  window.__rdioStrip = true;
  var apiTries = 0;
  function api() {
    try {
      var w = window.__TAURI__ && window.__TAURI__.window;
      return (w && w.getCurrentWindow) ? w : null;
    } catch (e) { return null; }
  }
  function build() {
    try {
      if (document.getElementById('rdio-top-strip')) return true;
      var root = document.documentElement || document.body;
      if (!root) return false;
      var strip = document.createElement('div');
      strip.id = 'rdio-top-strip';
      strip.style.cssText = 'position:fixed;top:0;left:0;right:0;height:28px;z-index:2147483647;display:flex;align-items:stretch;';
      var drag = document.createElement('div');
      drag.setAttribute('data-tauri-drag-region', '');
      drag.title = 'Drag to move';
      drag.style.cssText = 'flex:1;';
      var btns = document.createElement('div');
      btns.id = 'rdio-strip-btns';
      btns.style.cssText = 'display:flex;align-items:center;gap:4px;padding-right:6px;background:rgba(15,20,30,0.72);border-bottom-left-radius:8px;';
      function mkBtn(label, title, fn) {
        var b = document.createElement('button');
        b.type = 'button';
        b.textContent = label;
        b.title = title;
        b.style.cssText = 'width:28px;height:22px;border:none;border-radius:5px;background:rgba(255,255,255,0.10);color:#ffffff;font-size:14px;line-height:1;cursor:pointer;';
        b.addEventListener('click', function (e) {
          e.preventDefault(); e.stopPropagation();
          try {
            var r = fn();
            if (r && r.then) {
              r.then(function () {}, function (err) {
                b.textContent = 'ERR';
                try { b.title = String(err); } catch (e2) {}
              });
            }
          } catch (err) {
            b.textContent = 'ERR';
            try { b.title = String(err); } catch (e2) {}
          }
        });
        return b;
      }
      function withWin(fn) {
        var a = api();
        if (!a) return null;
        try { return fn(a.getCurrentWindow()); } catch (e) { return null; }
      }
      btns.appendChild(mkBtn('−', 'Minimize', function () {
        return withWin(function (w) { return w.minimize(); });
      }));
      btns.appendChild(mkBtn('✕', 'Close (tucks into tray)', function () {
        return withWin(function (w) { return w.close(); });
      }));
      strip.appendChild(drag);
      strip.appendChild(btns);
      root.appendChild(strip);
      (function markApi() {
        if (api()) return;
        if (++apiTries > 100) {
          btns.style.background = 'rgba(160,30,30,0.85)';
          return;
        }
        setTimeout(markApi, 100);
      })();
      return true;
    } catch (e) { return false; }
  }
  (function tick() {
    if (!build()) setTimeout(tick, 200);
  })();
  if (document.readyState === 'loading') {
    document.addEventListener('DOMContentLoaded', function () { build(); });
  }
  window.addEventListener('load', function () { build(); });
  setTimeout(function () { build(); }, 3000);
  // Whole-window drag: press and drag from any non-interactive part of
  // the page to move the window. The drag only engages after the pointer
  // moves a few pixels, so plain clicks (e.g. clicking a call to play it)
  // still land normally; buttons, inputs, sliders, links and media
  // controls are excluded entirely and keep their normal behavior.
  var INTERACTIVE_SEL = 'button, input, select, textarea, a, label, audio, video, [contenteditable], [role="button"], [role="slider"], [role="listbox"], [role="option"]';
  function isInteractiveTarget(t) {
    return !!(t && t.closest && t.closest(INTERACTIVE_SEL));
  }
  function inStrip(t) {
    return !!(t && t.closest && t.closest('#rdio-top-strip'));
  }
  var dragPending = null;
  document.addEventListener('mousedown', function (e) {
    if (e.button !== 0) return;
    if (isInteractiveTarget(e.target) || inStrip(e.target)) return;
    dragPending = { x: e.clientX, y: e.clientY };
  }, true);
  document.addEventListener('mousemove', function (e) {
    if (!dragPending) return;
    var dx = e.clientX - dragPending.x;
    var dy = e.clientY - dragPending.y;
    if (dx * dx + dy * dy < 16) return;
    dragPending = null;
    var a = api();
    if (!a) return;
    try {
      var p = a.getCurrentWindow().startDragging();
      if (p && p.catch) p.catch(function () {});
    } catch (err) {}
  }, true);
  document.addEventListener('mouseup', function () { dragPending = null; }, true);
  // Live title: mirror the scanner display's current call in the window
  // title and the tray tooltip ("Rdio Sherlock — <talkgroup>"), back to
  // the plain name when idle. Reads the v6 display DOM (.rdio-display);
  // on other page layouts it simply never fires and the title stays put.
  var BASE_TITLE = 'Rdio Sherlock';
  var lastTitle = BASE_TITLE;
  var lastTooltip = BASE_TITLE;
  var lastCallText = '';
  var trayIconPromise = null;
  function setAppTitle(t) {
    if (t !== BASE_TITLE) {
      lastCallText = t + ' · ' + new Date().toLocaleTimeString([], { hour: 'numeric', minute: '2-digit' });
    }
    // The window title goes back to the plain name when idle (per the
    // taskbar rule), but the tray tooltip then carries the last call
    // heard, so hovering the icon shows what was missed.
    var tooltip = (t === BASE_TITLE && lastCallText)
      ? BASE_TITLE + ' — last: ' + lastCallText
      : t;
    if (t === lastTitle && tooltip === lastTooltip) return;
    lastTitle = t;
    lastTooltip = tooltip;
    var a = api();
    if (!a) return;
    try {
      var p = a.getCurrentWindow().setTitle(t);
      if (p && p.catch) p.catch(function () {});
    } catch (err) {}
    try {
      if (window.__TAURI__.tray && window.__TAURI__.tray.TrayIcon) {
        if (!trayIconPromise) {
          trayIconPromise = window.__TAURI__.tray.TrayIcon.getById('tray');
        }
        trayIconPromise.then(function (icon) {
          if (icon) {
            var q = icon.setTooltip(tooltip);
            if (q && q.catch) q.catch(function () {});
          }
        }).catch(function () {});
      }
    } catch (err) {}
  }
  function readTitle() {
    var disp = document.querySelector('.rdio-display');
    if (!disp) return null;
    var big = disp.querySelector('.row.big span');
    var name = big ? (big.textContent || '').trim() : '';
    if (!name || /^rdio scanner/i.test(name)) return BASE_TITLE;
    var label = '';
    var tgid = '';
    var rows = disp.querySelectorAll('.row');
    try {
      var cell = rows[3] && rows[3].querySelector('div span');
      if (cell) label = (cell.textContent || '').trim();
      var rowText = rows[5] ? (rows[5].textContent || '') : '';
      var m = rowText.match(/TGID:\s*(\d+)/);
      if (m) tgid = m[1];
    } catch (e) {}
    // While a call plays, the talkgroup ID takes the app name's place
    // at the front, so the taskbar shows as much of the call as fits.
    var t = (tgid ? tgid + ' — ' : '') + name;
    if (label && label !== name) t += ' (' + label + ')';
    return t;
  }
  function tickTitle() {
    var t = readTitle();
    if (t !== null) setAppTitle(t);
  }
  try {
    var titleObserver = new MutationObserver(function () { tickTitle(); });
    var startTitleObs = function () {
      if (document.body) {
        titleObserver.observe(document.body, {
          childList: true, subtree: true, characterData: true
        });
      }
    };
    if (document.body) startTitleObs();
    else document.addEventListener('DOMContentLoaded', startTitleObs);
  } catch (e) {}
  setInterval(tickTitle, 2000);
  // Custom right-click menu: the browser default menu is suppressed
  // everywhere; instead a small menu offers Change server (straight to
  // the setup form, no auto-connect).
  var ctxMenu = null;
  var alwaysOnTop = false;
  function closeCtxMenu() {
    if (ctxMenu) { ctxMenu.remove(); ctxMenu = null; }
  }
  function mkCtxItem(label, onClick) {
    var el = document.createElement('div');
    el.textContent = label;
    el.style.cssText = 'padding:8px 12px;font-size:13px;color:#f2f2f2;border-radius:5px;cursor:pointer;user-select:none;';
    el.addEventListener('mouseenter', function () { el.style.background = 'rgba(255,255,255,0.10)'; });
    el.addEventListener('mouseleave', function () { el.style.background = 'transparent'; });
    el.addEventListener('click', function () { closeCtxMenu(); onClick(); });
    return el;
  }
  document.addEventListener('contextmenu', function (e) {
    e.preventDefault();
    closeCtxMenu();
    ctxMenu = document.createElement('div');
    ctxMenu.style.cssText = 'position:fixed;z-index:2147483647;background:#1e1e24;border:1px solid rgba(255,255,255,0.16);border-radius:8px;padding:4px;min-width:170px;box-shadow:0 10px 28px rgba(0,0,0,0.5);font-family:-apple-system,"Segoe UI",Roboto,sans-serif;';
    ctxMenu.style.left = Math.min(e.clientX, window.innerWidth - 190) + 'px';
    ctxMenu.style.top = Math.min(e.clientY, window.innerHeight - 110) + 'px';
    ctxMenu.appendChild(mkCtxItem('Change server…', function () {
      window.location.href = 'http://tauri.localhost/index.html#edit';
    }));
    ctxMenu.appendChild(mkCtxItem((alwaysOnTop ? '✓ ' : '') + 'Always on top', function () {
      alwaysOnTop = !alwaysOnTop;
      var a = api();
      if (!a) return;
      try {
        var p = a.getCurrentWindow().setAlwaysOnTop(alwaysOnTop);
        if (p && p.catch) p.catch(function () {});
      } catch (err) {}
    }));
    document.body.appendChild(ctxMenu);
  });
  document.addEventListener('mousedown', function (e) {
    if (ctxMenu && !ctxMenu.contains(e.target)) closeCtxMenu();
  }, true);
  document.addEventListener('keydown', function (e) {
    if (e.key === 'Escape') closeCtxMenu();
  });
  window.addEventListener('blur', closeCtxMenu);
})();"##;

fn refresh_tray_menu<R: Runtime>(app: &AppHandle<R>) -> tauri::Result<()> {
    let muted = MUTED.load(Ordering::SeqCst);

    let show = MenuItem::with_id(app, "show", "Show", true, None::<&str>)?;
    let minimize = MenuItem::with_id(app, "minimize", "Minimize", true, None::<&str>)?;
    let server = MenuItem::with_id(app, "server", "Change server…", true, None::<&str>)?;
    let mute = MenuItem::with_id(
        app,
        "mute",
        if muted { "Unmute" } else { "Mute" },
        true,
        None::<&str>,
    )?;
    let autostart = CheckMenuItem::with_id(
        app,
        "autostart",
        "Start on login",
        true,
        autostart_enabled(),
        None::<&str>,
    )?;
    let quit = MenuItem::with_id(app, "quit", "Quit", true, None::<&str>)?;

    let menu = Menu::with_items(app, &[&show, &minimize, &server, &mute, &autostart, &quit])?;
    if let Some(tray) = app.tray_by_id("tray") {
        let _ = tray.set_menu(Some(menu));
    }
    Ok(())
}

/// Start-on-login state: an "Rdio Sherlock" value under the per-user
/// Run key pointing at the current exe. Enabled only when the stored
/// path is this exe, so moving the portable exe reads as off until the
/// user toggles it again.
const RUN_KEY: &str = r"HKCU\Software\Microsoft\Windows\CurrentVersion\Run";
const AUTOSTART_NAME: &str = "Rdio Sherlock";

fn autostart_enabled() -> bool {
    #[cfg(windows)]
    {
        use std::os::windows::process::CommandExt;
        if let Ok(out) = Command::new("reg")
            .args(["query", RUN_KEY, "/v", AUTOSTART_NAME])
            .creation_flags(0x0800_0000)
            .output()
        {
            if out.status.success() {
                if let Ok(exe) = std::env::current_exe() {
                    return String::from_utf8_lossy(&out.stdout)
                        .contains(&exe.to_string_lossy().into_owned());
                }
            }
        }
    }
    false
}

fn set_autostart(enable: bool) {
    #[cfg(windows)]
    {
        use std::os::windows::process::CommandExt;
        if enable {
            if let Ok(exe) = std::env::current_exe() {
                let value = format!("\"{}\"", exe.display());
                let _ = Command::new("reg")
                    .args([
                        "add",
                        RUN_KEY,
                        "/v",
                        AUTOSTART_NAME,
                        "/t",
                        "REG_SZ",
                        "/d",
                        &value,
                        "/f",
                    ])
                    .creation_flags(0x0800_0000)
                    .output();
            }
        } else {
            let _ = Command::new("reg")
                .args(["delete", RUN_KEY, "/v", AUTOSTART_NAME, "/f"])
                .creation_flags(0x0800_0000)
                .output();
        }
    }
    #[cfg(not(windows))]
    let _ = enable;
}

/// Window geometry memory: last position/size (logical units), saved to
/// geometry.json in the app data dir and restored on the next launch.
#[derive(Clone, Copy)]
struct Geometry {
    x: f64,
    y: f64,
    w: f64,
    h: f64,
}

fn geometry_path<R: Runtime>(app: &AppHandle<R>) -> Option<std::path::PathBuf> {
    app.path().app_data_dir().ok().map(|d| d.join("geometry.json"))
}

fn load_geometry<R: Runtime>(app: &AppHandle<R>) -> Option<Geometry> {
    let text = std::fs::read_to_string(geometry_path(app)?).ok()?;
    let get = |key: &str| -> Option<f64> {
        let pat = format!("\"{}\":", key);
        let start = text.find(&pat)? + pat.len();
        let rest = &text[start..];
        let end = rest
            .find(|c: char| !(c.is_ascii_digit() || c == '.' || c == '-'))
            .unwrap_or(rest.len());
        rest[..end].trim().parse::<f64>().ok()
    };
    let g = Geometry {
        x: get("x")?,
        y: get("y")?,
        w: get("w")?,
        h: get("h")?,
    };
    if g.w >= 400.0 && g.h >= 300.0 && g.x.abs() < 10000.0 && g.y.abs() < 10000.0 {
        Some(g)
    } else {
        None
    }
}

fn save_geometry<R: Runtime>(app: &AppHandle<R>, g: Geometry) {
    if let Some(path) = geometry_path(app) {
        if let Some(dir) = path.parent() {
            let _ = std::fs::create_dir_all(dir);
        }
        let text = format!(
            "{{\"x\":{},\"y\":{},\"w\":{},\"h\":{}}}",
            g.x, g.y, g.w, g.h
        );
        let _ = std::fs::write(path, text);
    }
}

static LAST_GEO_SAVE: Mutex<Option<Instant>> = Mutex::new(None);

fn save_window_geometry<R: Runtime>(window: &Window<R>) {
    if window.is_minimized().unwrap_or(false) {
        return;
    }
    {
        let mut last = LAST_GEO_SAVE.lock().unwrap();
        if let Some(t) = *last {
            if t.elapsed() < std::time::Duration::from_millis(250) {
                return;
            }
        }
        *last = Some(Instant::now());
    }
    let scale = window.scale_factor().unwrap_or(1.0);
    if let (Ok(pos), Ok(size)) = (window.outer_position(), window.inner_size()) {
        let w = size.width as f64 / scale;
        let h = size.height as f64 / scale;
        if w < 400.0 || h < 300.0 {
            return;
        }
        save_geometry(
            window.app_handle(),
            Geometry {
                x: pos.x as f64 / scale,
                y: pos.y as f64 / scale,
                w,
                h,
            },
        );
    }
}

fn main() {
    tauri::Builder::default()
        .plugin(tauri_plugin_single_instance::init(|app, _args, _cwd| {
            if let Some(window) = app.get_webview_window("main") {
                let _ = window.show();
                let _ = window.set_focus();
            }
        }))
        .setup(|app| {
            let mut wb = WebviewWindowBuilder::new(
                app,
                "main",
                WebviewUrl::App("index.html".into()),
            )
            .title("Rdio Sherlock")
            .decorations(false)
            .initialization_script(INIT_SCRIPT)
            .initialization_script(PIN_SEED_SCRIPT);
            if let Some(g) = load_geometry(app.handle()) {
                wb = wb.inner_size(g.w, g.h).position(g.x, g.y);
            } else {
                wb = wb.inner_size(1280.0, 800.0);
            }
            let _main = wb.build()?;

            let mut builder = TrayIconBuilder::with_id("tray")
                .tooltip("Rdio Sherlock")
                .show_menu_on_left_click(false)
                .on_menu_event(move |app, event| match event.id.as_ref() {
                    "show" => {
                        if let Some(window) = app.get_webview_window("main") {
                            let _ = window.show();
                            let _ = window.set_focus();
                        }
                    }
                    "minimize" => {
                        if let Some(window) = app.get_webview_window("main") {
                            let _ = window.minimize();
                        }
                    }
                    "server" => {
                        if let Some(window) = app.get_webview_window("main") {
                            let _ = window.show();
                            let _ = window.set_focus();
                            // Back to the bundled setup page, straight to
                            // the form (#edit skips the auto-connect) —
                            // Windows serves app pages at
                            // http://tauri.localhost/.
                            let _ = window.eval(
                                "window.location.href='http://tauri.localhost/index.html#edit';",
                            );
                        }
                    }
                    "mute" => {
                        let muted = !MUTED.load(Ordering::SeqCst);
                        MUTED.store(muted, Ordering::SeqCst);
                        if let Some(window) = app.get_webview_window("main") {
                            let _ = window.eval(if muted { MUTE_JS } else { UNMUTE_JS });
                        }
                        let _ = refresh_tray_menu(app);
                    }
                    "autostart" => {
                        set_autostart(!autostart_enabled());
                        let _ = refresh_tray_menu(app);
                    }
                    "quit" => app.exit(0),
                    _ => {}
                })
                .on_tray_icon_event(|tray, event| {
                    if let TrayIconEvent::Click {
                        button: MouseButton::Left,
                        ..
                    } = event
                    {
                        let app = tray.app_handle();
                        if let Some(window) = app.get_webview_window("main") {
                            let _ = window.show();
                            let _ = window.set_focus();
                        }
                    }
                });
            if let Some(icon) = app.default_window_icon() {
                builder = builder.icon(icon.clone());
            }
            let _tray = builder.build(app)?;
            let _ = refresh_tray_menu(app.handle());
            Ok(())
        })
        .on_window_event(|window, event| {
            if window.label() != "main" {
                return;
            }
            match event {
                WindowEvent::CloseRequested { api, .. } => {
                    save_window_geometry(window);
                    let _ = window.hide();
                    api.prevent_close();
                }
                WindowEvent::Moved(_) | WindowEvent::Resized(_) => {
                    save_window_geometry(window);
                }
                _ => {}
            }
        })
        .run(tauri::generate_context!())
        .expect("error while running tauri application");
}
