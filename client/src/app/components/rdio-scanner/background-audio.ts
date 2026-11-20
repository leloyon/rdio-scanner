/*
 * Phone helpers: lock-screen controls (Media Session), keep-screen-awake (Wake Lock)
 * and a silent keep-alive audio element so the browser treats the page as a media
 * player and keeps audio going with the screen off. Everything is feature-detected
 * and best-effort; iOS may still suspend a page after a long time or in Low Power Mode.
 */

export interface BackgroundAudioHandlers {
    pause: () => void;
    play: () => void;
    next: () => void;
    previous: () => void;
}

// 0.5s of 8kHz mono silence (PCM WAV), looped.
function silentWavUrl(): string {
    const samples = 4000;
    const buf = new ArrayBuffer(44 + samples);
    const v = new DataView(buf);
    const w = (o: number, s: string) => { for (let i = 0; i < s.length; i++) v.setUint8(o + i, s.charCodeAt(i)); };
    w(0, 'RIFF'); v.setUint32(4, 36 + samples, true); w(8, 'WAVE'); w(12, 'fmt ');
    v.setUint32(16, 16, true); v.setUint16(20, 1, true); v.setUint16(22, 1, true);
    v.setUint32(24, 8000, true); v.setUint32(28, 8000, true); v.setUint16(32, 1, true); v.setUint16(34, 8, true);
    w(36, 'data'); v.setUint32(40, samples, true);
    for (let i = 0; i < samples; i++) v.setUint8(44 + i, 128);
    return URL.createObjectURL(new Blob([buf], { type: 'audio/wav' }));
}

export class BackgroundAudio {
    private el: HTMLAudioElement | undefined;
    private lock: any;
    private wantLock = false;
    private active = false;
    keepAlive = false;
    private handlers: BackgroundAudioHandlers | undefined;

    constructor() {
        try {
            document.addEventListener('visibilitychange', () => {
                if (document.visibilityState === 'visible') {
                    if (this.wantLock) this.acquireLock();
                    if (this.active && this.el?.paused) this.el.play().catch(() => { });
                }
            });
        } catch { }
    }

    get wakeLockSupported(): boolean {
        return typeof navigator !== 'undefined' && 'wakeLock' in navigator;
    }

    setHandlers(h: BackgroundAudioHandlers): void {
        this.handlers = h;
        const ms = (navigator as any).mediaSession;
        if (!ms) return;
        const set = (a: string, fn: () => void) => { try { ms.setActionHandler(a, fn); } catch { } };
        set('play', () => this.handlers?.play());
        set('pause', () => this.handlers?.pause());
        set('nexttrack', () => this.handlers?.next());
        set('previoustrack', () => this.handlers?.previous());
    }

    /** Call from a user gesture (the livefeed button) so iOS allows playback. */
    start(): void {
        this.active = true;
        if (!this.keepAlive) { this.setState(false); return; }
        try {
            const nav = navigator as any;
            if (nav.audioSession) nav.audioSession.type = 'playback';
        } catch { }
        try {
            if (!this.el) {
                this.el = new Audio(silentWavUrl());
                this.el.loop = true;
                this.el.setAttribute('playsinline', '');
            }
            this.el.play().catch(() => { });
        } catch { }
        this.setState(false);
    }

    stop(): void {
        this.active = false;
        try { this.el?.pause(); } catch { }
        try {
            const ms = (navigator as any).mediaSession;
            if (ms) { ms.metadata = null; ms.playbackState = 'none'; }
        } catch { }
    }

    setState(paused: boolean): void {
        try {
            const ms = (navigator as any).mediaSession;
            if (ms) ms.playbackState = this.active ? (paused ? 'paused' : 'playing') : 'none';
        } catch { }
        if (this.active && this.el) {
            if (paused) { /* keep the element running so the controls stay on the lock screen */ }
            else if (this.el.paused) this.el.play().catch(() => { });
        }
    }

    setMetadata(title: string, artist: string, album: string): void {
        try {
            const ms = (navigator as any).mediaSession;
            const MM = (window as any).MediaMetadata;
            if (ms && MM) ms.metadata = new MM({ title, artist, album });
        } catch { }
    }

    setKeepAlive(on: boolean): void {
        this.keepAlive = on;
        if (!on) { try { this.el?.pause(); } catch { } }
        else if (this.active) this.start();
    }

    setWakeLock(on: boolean): void {
        this.wantLock = on;
        if (on) this.acquireLock();
        else {
            try { this.lock?.release(); } catch { }
            this.lock = undefined;
        }
    }

    private acquireLock(): void {
        if (!this.wakeLockSupported) return;
        try {
            (navigator as any).wakeLock.request('screen').then((l: any) => {
                this.lock = l;
                l.addEventListener?.('release', () => { if (this.lock === l) this.lock = undefined; });
            }).catch(() => { });
        } catch { }
    }
}

export const backgroundAudio = new BackgroundAudio();
