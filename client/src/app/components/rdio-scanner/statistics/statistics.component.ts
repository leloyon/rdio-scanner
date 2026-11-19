/*
 * v4: Statistics dashboard (shared by Admin and, when enabled, the main scanner).
 * Read-only; the server caches results for 5 minutes.
 */

import { Component, Input, NgModule, OnDestroy } from '@angular/core';
import { AppSharedModule } from '../../../shared/shared.module';

interface TopTalkgroup {
    system: number;
    systemLabel: string;
    talkgroup: number;
    label: string;
    name: string;
    count: number;
}

interface Stats {
    range?: string;
    total: number;
    previous: number;
    hourly: number[];
    usual?: number[];
    peakHour: number;
    top: TopTalkgroup[];
    heatmap: number[][];
    heatTop: TopTalkgroup[][][];
    heatDates: string[];
    systems: { id: number; label: string }[];
    cachedAt: string;
}

@Component({
    selector: 'rdio-scanner-statistics',
    template: `
<div class="stats" [class.admin]="admin">
    <div class="filters">
        <mat-form-field class="scanner-theme-control">
            <mat-label>Time range</mat-label>
            <mat-select [panelClass]="admin ? '' : 'scanner-theme-menu'" [(value)]="range" (selectionChange)="load()">
                <mat-option *ngFor="let r of ranges" [value]="r.v">{{ r.l }}</mat-option>
            </mat-select>
        </mat-form-field>
        <mat-form-field class="scanner-theme-control">
            <mat-label>System</mat-label>
            <mat-select [panelClass]="admin ? '' : 'scanner-theme-menu'" [(value)]="system" (selectionChange)="load()">
                <mat-option [value]="0">All systems</mat-option>
                <mat-option *ngFor="let s of stats?.systems" [value]="s.id">{{ s.label }}</mat-option>
            </mat-select>
        </mat-form-field>
        <span class="updated" *ngIf="stats?.cachedAt">Updated {{ updatedAgo() }} · next update {{ nextUpdate() }}</span>
    </div>

    <div *ngIf="error" class="error">{{ error }}</div>
    <div *ngIf="loading">Loading…</div>

    <ng-container *ngIf="stats && !loading">
        <div class="cards">
            <div class="card"><div class="k">Total calls</div><div class="v">{{ stats.total | number }}</div></div>
            <div class="card"><div class="k">Previous period</div><div class="v">{{ stats.previous | number }}</div>
                <div class="d" *ngIf="stats.previous">{{ change() }}</div></div>
            <div class="card"><div class="k">Busiest talkgroup</div><div class="v small">{{ stats.top[0]?.name || stats.top[0]?.label || '—' }}</div></div>
            <div class="card"><div class="k">Peak hour</div><div class="v">{{ stats.total ? hourLabel(stats.peakHour) : '—' }}</div></div>
        </div>

        <h3>Calls per hour <span class="sub" *ngIf="chartNote()">· {{ chartNote() }}</span></h3>
        <div class="chart">
            <div class="yaxis">
                <span *ngFor="let t of ticks()" [style.bottom.%]="t / axisMax() * 100">{{ t }}</span>
            </div>
            <div class="plot">
                <div class="grid" *ngFor="let t of ticks()" [style.bottom.%]="t / axisMax() * 100"></div>
                <div class="bar" *ngFor="let c of stats.hourly; let h = index" [class.now]="h === nowHour()" [title]="hourLabel(h) + ': ' + c + ' calls' + (showUsual() ? ' (usual ' + (stats.usual?.[h] ?? 0) + ')' : '') + (h === nowHour() ? ' (current hour)' : '') + (stats.range === '24h' && h > nowHour() ? ' (yesterday)' : '')">
                    <div class="fill" [style.height.%]="c / axisMax() * 100" [style.background-color]="heatColor(c, barMax())"></div>
                    <div class="usual" *ngIf="showUsual() && (stats.usual?.[h] ?? 0) > 0" [style.bottom.%]="(stats.usual?.[h] ?? 0) / axisMax() * 100"></div>
                    <span class="x" [class.now]="h === nowHour()">{{ h }}</span>
                </div>
            </div>
        </div>

        <h3>Top 10 talkgroups</h3>
        <div *ngIf="selectedDay !== null" class="selection">
            {{ stats.heatDates[selectedDay] }} · {{ hourLabel(selectedHour) }}–{{ hourLabel(selectedHour + 1) }}
            <button mat-stroked-button (click)="clearSelection()">Reset</button>
        </div>
        <div *ngIf="!displayTop().length">No calls in this period.</div>
        <div class="top" *ngFor="let t of displayTop(); let i = index">
            <span class="rank">{{ i + 1 }}</span>
            <span class="name">{{ t.name || t.label }} <small>{{ t.systemLabel }} · TG {{ t.talkgroup }}</small></span>
            <span class="meter"><span [style.width.%]="t.count / displayTop()[0].count * 100" [style.background-color]="heatColor(t.count, displayTop()[0].count)"></span></span>
            <span class="count">{{ t.count | number }}</span>
        </div>

        <h3>Busiest days &amp; hours (last 7 days)</h3>
        <div class="heat">
            <div class="hrow head"><span></span><span *ngFor="let h of hours">{{ h % 3 === 0 ? h : '' }}</span></div>
            <div class="hrow" *ngFor="let d of days; let di = index" [class.today]="isToday(di)">
                <span class="day" [class.today]="isToday(di)" [title]="isToday(di) ? 'Today' : ''">{{ d }}</span>
                <button mat-button *ngFor="let c of stats.heatmap[di]; let h = index" class="cell"
                    (click)="selectCell(di, h)" [class.selected]="selectedDay === di && selectedHour === h"
                    [attr.aria-pressed]="selectedDay === di && selectedHour === h"
                    [attr.aria-label]="stats.heatDates[di] + ' ' + hourLabel(h) + ': ' + c + ' calls'"
                    [style.background-color]="heatColor(c, heatMax())" [title]="stats.heatDates[di] + ' ' + hourLabel(h) + ': ' + c + ' calls'"></button>
            </div>
        </div>
        <p class="note">Ranked by number of calls. Updated at most every 5 minutes.</p>
    </ng-container>
</div>`,
    styles: [`
.stats { padding: 4px 8px; max-width: 900px; }
.stats h3 { margin: 6px 0 4px; font-size: 15px; }
.stats.admin { --scanner-control-surface: #fff; --scanner-control-ink: rgba(0, 0, 0, 0.87); }
.stats.admin .filters mat-form-field { border: 1px solid rgba(0, 0, 0, .3); }
.stats { --filter-surface: var(--scanner-control-surface, rgb(30, 30, 30)); --filter-ink: var(--scanner-control-ink, #fff); }
.filters mat-form-field { background: var(--filter-surface); color: var(--filter-ink); padding: 6px 8px 0; border-radius: 3px; }
.filters { display: flex; gap: 12px; flex-wrap: wrap; align-items: center; }
.cards { display: grid; grid-template-columns: repeat(auto-fit, minmax(150px, 1fr)); gap: 8px; margin: 6px 0 4px; }
.card { border: 1px solid rgba(128,128,128,.4); border-radius: 6px; padding: 3px 10px; }
.card .k { font-size: 12px; opacity: .7; }
.card .v { font-size: 20px; font-weight: 600; line-height: 1.15; }
.card .v.small { font-size: 15px; }
.card .d { font-size: 12px; opacity: .8; }
.chart { display: flex; height: clamp(90px, 15vh, 160px); margin-bottom: 20px; }
.yaxis { position: relative; width: 40px; font-size: 10px; }
.yaxis span { position: absolute; right: 4px; transform: translateY(50%); }
.plot { position: relative; flex: 1; display: flex; align-items: flex-end; gap: 2px; border-left: 1px solid rgba(128,128,128,.5); border-bottom: 1px solid rgba(128,128,128,.5); }
.grid { position: absolute; left: 0; right: 0; border-top: 1px dashed rgba(128,128,128,.2); }
.bar { position: relative; flex: 1; height: 100%; display: flex; align-items: flex-end; }
.fill { width: 100%; background: currentColor; border-radius: 2px 2px 0 0; }
.sub { font-size: 12px; font-weight: 400; opacity: .7; }
.usual { position: absolute; left: 6%; right: 6%; height: 0; border-top: 2px solid currentColor; opacity: .65; pointer-events: none; }
.updated { font-size: 12px; opacity: .7; margin-left: 8px; }
.x.now { background: #1e88e5; color: #fff; font-weight: 700; border-radius: 3px; }
.x { position: absolute; bottom: -16px; width: 100%; text-align: center; font-size: 9px; }
.top { display: flex; align-items: center; gap: 8px; padding: 4px 0; }
.rank { width: 20px; text-align: right; opacity: .7; }
.name { width: 45%; overflow: hidden; text-overflow: ellipsis; white-space: nowrap; }
.name small { opacity: .6; }
.meter { flex: 1; height: 8px; background: rgba(128,128,128,.2); border-radius: 4px; overflow: hidden; }
.meter span { display: block; height: 100%; }
.count { width: 60px; text-align: right; }
.heat { overflow-x: auto; padding: 3px 6px 3px 3px; }
.hrow { display: grid; grid-template-columns: 36px repeat(24, minmax(12px, 1fr)); gap: 2px; margin-bottom: 2px; }
.hrow.head span { font-size: 9px; text-align: center; }
.day { font-size: 11px; }
.day.today { background: #1e88e5; color: #fff; font-weight: 700; border-radius: 3px; text-align: center; }
.hrow.today { background: rgba(30, 136, 229, .12); border-radius: 3px; }
.cell.mat-button { height: clamp(12px, 2.2vh, 18px); min-width: 0; padding: 0; line-height: 1; border-radius: 2px; }
.cell.selected { outline: 2px solid currentColor; outline-offset: 1px; }
.selection { display: flex; align-items: center; gap: 12px; margin-bottom: 8px; flex-wrap: wrap; }
.note { font-size: 11px; opacity: .6; margin: 4px 0 0; }
.error { color: #e53935; }
`],
})
export class RdioScannerStatisticsComponent implements OnDestroy {
    @Input() admin = false;

    days = ['Sun', 'Mon', 'Tue', 'Wed', 'Thu', 'Fri', 'Sat'];
    hours = Array.from({ length: 24 }, (_, i) => i);
    ranges = [
        { v: 'today', l: 'Today' },
        { v: 'yesterday', l: 'Yesterday' },
        { v: '24h', l: 'Last 24 hours' },
        { v: '7d', l: 'Last 7 days' },
        { v: '30d', l: 'Last 30 days' },
    ];
    range = '24h';
    system = 0;
    stats: Stats | undefined;
    private clock = Date.now();
    private busy = false;
    private clockTimer = setInterval(() => { this.clock = Date.now(); this.autoRefresh(); }, 15000);
    loading = false;
    error = '';
    selectedDay: number | null = null;
    selectedHour = 0;
    selectCell(day: number, hour: number): void { this.selectedDay = day; this.selectedHour = hour; }
    clearSelection(): void { this.selectedDay = null; }
    displayTop(): TopTalkgroup[] {
        return this.selectedDay === null ? (this.stats?.top || []) : (this.stats?.heatTop[this.selectedDay][this.selectedHour] || []);
    }

    ngOnDestroy(): void {
        clearInterval(this.clockTimer);
    }

    // "Updated 3 min ago", from the server's own timestamp for the cached result.
    updatedAgo(): string {
        const t = Date.parse(this.stats?.cachedAt || '');
        if (isNaN(t)) return '';
        const min = Math.floor(Math.max(0, this.clock - t) / 60000);
        return min < 1 ? 'just now' : min === 1 ? '1 min ago' : `${min} min ago`;
    }

    // The server keeps each result for 5 minutes; this is when the next one is available.
    private static readonly refreshMs = 5 * 60 * 1000;

    nextUpdate(): string {
        const t = Date.parse(this.stats?.cachedAt || '');
        if (isNaN(t)) return '';
        const ms = t + RdioScannerStatisticsComponent.refreshMs - this.clock;
        if (ms <= 0) return 'any moment';
        const min = Math.ceil(ms / 60000);
        return min <= 1 ? 'in about a minute' : `in about ${min} min`;
    }

    // Quietly reloads once the cached result has expired (no "Loading…", keeps the selected heatmap cell).
    private autoRefresh(): void {
        const t = Date.parse(this.stats?.cachedAt || '');
        if (this.loading || this.busy || isNaN(t) || this.clock < t + RdioScannerStatisticsComponent.refreshMs + 2000) return;
        this.load(true);
    }

    // Usual-hour markers only make sense when the bars are a single day's counts.
    showUsual(): boolean {
        return !!this.stats?.usual && ['today', 'yesterday', '24h'].includes(this.stats.range || '');
    }

    chartNote(): string {
        const parts: string[] = [];
        if (this.stats?.range === '24h') parts.push('last 24 hours');
        if (this.showUsual()) parts.push('line = usual for that hour (average of the previous 6 days)');
        return parts.join(' · ');
    }

    async load(silent = false): Promise<void> {
        if (silent) {
            if (this.busy) return;
        } else {
            this.clearSelection();
            this.loading = true;
            this.error = '';
        }
        this.busy = true;
        const range = this.range;
        const system = this.system;
        try {
            const base = window.location.href.replace(/[#?].*$/, '').replace(/\/+$/, '').replace(/\/admin$/, '');
            const headers: Record<string, string> = {};
            const token = window.sessionStorage?.getItem('rdio-scanner-admin-token');
            if (token) headers['Authorization'] = token;
            const res = await fetch(`${base}/api/statistics?range=${range}&system=${system}`, { headers });
            if (!res.ok) throw new Error(res.status === 401 ? 'Statistics are not available.' : `Error ${res.status}`);
            const data = await res.json();
            // Ignore a quiet update that finished after the user changed the range or system.
            if (!silent || (range === this.range && system === this.system)) this.stats = data;
        } catch (e: any) {
            if (!silent) this.error = e?.message || 'Unable to load statistics';
        }
        this.busy = false;
        if (!silent) this.loading = false;
    }

    axisMax(): number {
        const m = Math.max(0, ...(this.stats?.hourly || [0]), ...(this.showUsual() ? (this.stats?.usual || []) : []));
        return Math.max(10, Math.ceil(m / 10) * 10);
    }

    // Y axis always counts in multiples of 10; labels thin out only on very busy systems.
    ticks(): number[] {
        const max = this.axisMax();
        const step = 10 * Math.max(1, Math.ceil(max / 100));
        const t: number[] = [];
        for (let v = 0; v <= max; v += step) t.push(v);
        return t;
    }

    heatMax(): number {
        return Math.max(1, ...(this.stats?.heatmap || []).map((r) => Math.max(...r)));
    }

    barMax(): number {
        return Math.max(1, ...(this.stats?.hourly || [0]));
    }

    // Heat scale: pale yellow (few calls) -> orange -> dark red (most calls).
    heatColor(c: number, max: number): string {
        if (!c) return 'rgba(128, 128, 128, 0.18)';
        const stops = [[255, 236, 139], [255, 167, 38], [229, 57, 53], [139, 0, 0]];
        const t = Math.min(1, Math.max(0, c / Math.max(1, max))) * (stops.length - 1);
        const i = Math.min(stops.length - 2, Math.floor(t));
        const f = t - i;
        const rgb = stops[i].map((v, k) => Math.round(v + (stops[i + 1][k] - v) * f));
        return `rgb(${rgb[0]}, ${rgb[1]}, ${rgb[2]})`;
    }

    // "Now" in the server's timezone: stats are bucketed server-side, so take the UTC offset from
    // the server's timestamp (cachedAt) and apply it to the browser clock.
    private serverNow(): Date {
        const m = /([+-])(\d{2}):?(\d{2})$/.exec(this.stats?.cachedAt || '');
        const offMin = m ? (m[1] === '-' ? -1 : 1) * (parseInt(m[2], 10) * 60 + parseInt(m[3], 10)) : 0;
        // Fields read with getUTC*() then equal the server's local date/time.
        return new Date(Date.now() + offMin * 60000);
    }

    nowHour(): number {
        return this.stats ? this.serverNow().getUTCHours() : -1;
    }

    isToday(row: number): boolean {
        if (!this.stats) return false;
        return this.stats.heatDates[row] === this.serverNow().toISOString().slice(0, 10);
    }

    hourLabel(h: number): string {
        return `${String(h).padStart(2, '0')}:00`;
    }

    change(): string {
        if (!this.stats?.previous) return '';
        const p = Math.round(((this.stats.total - this.stats.previous) / this.stats.previous) * 100);
        return `${p >= 0 ? '+' : ''}${p}% vs previous`;
    }
}

@NgModule({
    declarations: [RdioScannerStatisticsComponent],
    exports: [RdioScannerStatisticsComponent],
    imports: [AppSharedModule],
})
export class RdioScannerStatisticsModule { }
