/*
 * v4: Statistics dashboard (shared by Admin and, when enabled, the main scanner).
 * Read-only; the server caches results for 5 minutes.
 */

import { Component, Input, NgModule } from '@angular/core';
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
    total: number;
    previous: number;
    hourly: number[];
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
<div class="stats">
    <div class="filters">
        <mat-form-field class="scanner-theme-control">
            <mat-label>Time range</mat-label>
            <mat-select panelClass="scanner-theme-menu" [(value)]="range" (selectionChange)="load()">
                <mat-option *ngFor="let r of ranges" [value]="r.v">{{ r.l }}</mat-option>
            </mat-select>
        </mat-form-field>
        <mat-form-field class="scanner-theme-control">
            <mat-label>System</mat-label>
            <mat-select panelClass="scanner-theme-menu" [(value)]="system" (selectionChange)="load()">
                <mat-option [value]="0">All systems</mat-option>
                <mat-option *ngFor="let s of stats?.systems" [value]="s.id">{{ s.label }}</mat-option>
            </mat-select>
        </mat-form-field>
        <button mat-stroked-button (click)="load()">Refresh</button>
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

        <h3>Calls per hour</h3>
        <div class="chart">
            <div class="yaxis">
                <span *ngFor="let t of ticks()" [style.bottom.%]="t / axisMax() * 100">{{ t }}</span>
            </div>
            <div class="plot">
                <div class="grid" *ngFor="let t of ticks()" [style.bottom.%]="t / axisMax() * 100"></div>
                <div class="bar" *ngFor="let c of stats.hourly; let h = index" [title]="hourLabel(h) + ': ' + c + ' calls'">
                    <div class="fill" [style.height.%]="c / axisMax() * 100" [style.background-color]="heatColor(c, barMax())"></div>
                    <span class="x">{{ h }}</span>
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
            <div class="hrow" *ngFor="let d of days; let di = index">
                <span class="day">{{ d }}</span>
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
.stats { padding: 8px; max-width: 900px; }
.stats { --filter-surface: var(--scanner-control-surface, rgb(30, 30, 30)); --filter-ink: var(--scanner-control-ink, #fff); }
.filters mat-form-field { background: var(--filter-surface); color: var(--filter-ink); padding: 6px 8px 0; border-radius: 3px; }
.filters { display: flex; gap: 12px; flex-wrap: wrap; align-items: center; }
.cards { display: grid; grid-template-columns: repeat(auto-fit, minmax(150px, 1fr)); gap: 8px; margin: 8px 0 16px; }
.card { border: 1px solid rgba(128,128,128,.4); border-radius: 6px; padding: 10px; }
.card .k { font-size: 12px; opacity: .7; }
.card .v { font-size: 24px; font-weight: 600; }
.card .v.small { font-size: 16px; }
.card .d { font-size: 12px; opacity: .8; }
.chart { display: flex; height: 220px; margin-bottom: 24px; }
.yaxis { position: relative; width: 40px; font-size: 10px; }
.yaxis span { position: absolute; right: 4px; transform: translateY(50%); }
.plot { position: relative; flex: 1; display: flex; align-items: flex-end; gap: 2px; border-left: 1px solid rgba(128,128,128,.5); border-bottom: 1px solid rgba(128,128,128,.5); }
.grid { position: absolute; left: 0; right: 0; border-top: 1px dashed rgba(128,128,128,.2); }
.bar { position: relative; flex: 1; height: 100%; display: flex; align-items: flex-end; }
.fill { width: 100%; background: currentColor; border-radius: 2px 2px 0 0; }
.x { position: absolute; bottom: -16px; width: 100%; text-align: center; font-size: 9px; }
.top { display: flex; align-items: center; gap: 8px; padding: 4px 0; }
.rank { width: 20px; text-align: right; opacity: .7; }
.name { width: 45%; overflow: hidden; text-overflow: ellipsis; white-space: nowrap; }
.name small { opacity: .6; }
.meter { flex: 1; height: 8px; background: rgba(128,128,128,.2); border-radius: 4px; overflow: hidden; }
.meter span { display: block; height: 100%; }
.count { width: 60px; text-align: right; }
.heat { overflow-x: auto; }
.hrow { display: grid; grid-template-columns: 36px repeat(24, minmax(12px, 1fr)); gap: 2px; margin-bottom: 2px; }
.hrow.head span { font-size: 9px; text-align: center; }
.day { font-size: 11px; }
.cell.mat-button { height: 20px; min-width: 0; padding: 0; line-height: 20px; border-radius: 2px; }
.cell.selected { outline: 2px solid currentColor; outline-offset: 1px; }
.selection { display: flex; align-items: center; gap: 12px; margin-bottom: 8px; flex-wrap: wrap; }
.note { font-size: 11px; opacity: .6; margin-top: 12px; }
.error { color: #e53935; }
`],
})
export class RdioScannerStatisticsComponent {
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
    loading = false;
    error = '';
    selectedDay: number | null = null;
    selectedHour = 0;
    selectCell(day: number, hour: number): void { this.selectedDay = day; this.selectedHour = hour; }
    clearSelection(): void { this.selectedDay = null; }
    displayTop(): TopTalkgroup[] {
        return this.selectedDay === null ? (this.stats?.top || []) : (this.stats?.heatTop[this.selectedDay][this.selectedHour] || []);
    }

    async load(): Promise<void> {
        this.clearSelection();
        this.loading = true;
        this.error = '';
        try {
            const base = window.location.href.replace(/[#?].*$/, '').replace(/\/+$/, '').replace(/\/admin$/, '');
            const headers: Record<string, string> = {};
            const token = window.sessionStorage?.getItem('rdio-scanner-admin-token');
            if (token) headers['Authorization'] = token;
            const res = await fetch(`${base}/api/statistics?range=${this.range}&system=${this.system}`, { headers });
            if (!res.ok) throw new Error(res.status === 401 ? 'Statistics are not available.' : `Error ${res.status}`);
            this.stats = await res.json();
        } catch (e: any) {
            this.error = e?.message || 'Unable to load statistics';
        }
        this.loading = false;
    }

    axisMax(): number {
        const m = Math.max(0, ...(this.stats?.hourly || [0]));
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
