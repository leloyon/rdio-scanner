/*
 * v4: Talkgroup maintenance tools. Always previews first; deleting requires
 * a confirmation. Named talkgroups are never touched and recordings are kept.
 */

import { Component } from '@angular/core';

interface PruneResult {
    mode: string;
    confirmed: boolean;
    count: number;
    talkgroups: { system: number; systemLabel: string; talkgroup: number }[];
}

@Component({
    selector: 'rdio-scanner-admin-prune-talkgroups',
    template: `
<div class="prune">
    <p class="mat-caption">
        Only talkgroups that were never given a name (label is just the number) can be removed.
        Named talkgroups are never touched, and past call recordings are kept.
    </p>
    <div class="buttons">
        <button mat-stroked-button (click)="preview('phantom')" [disabled]="busy">Prune phantom talkgroups</button>
        <button mat-stroked-button (click)="preview('unnamed')" [disabled]="busy">Prune all unnamed talkgroups</button>
    </div>
    <p class="mat-caption">Phantom = unnamed and ID below 100 or above 65535 (usually radio/unit IDs or decode errors).</p>

    <div *ngIf="message" class="msg">{{ message }}</div>

    <div *ngIf="result && !result.confirmed">
        <p><b>{{ result.count }}</b> {{ result.mode }} talkgroup(s) found.</p>
        <div class="list" *ngIf="result.count">
            <span *ngFor="let t of result.talkgroups">{{ t.systemLabel }}: {{ t.talkgroup }}</span>
        </div>
        <div class="buttons" *ngIf="result.count">
            <button mat-flat-button color="warn" (click)="confirm()" [disabled]="busy">Delete {{ result.count }} talkgroup(s)</button>
            <button mat-button (click)="result = undefined">Cancel</button>
        </div>
    </div>
</div>`,
    styles: [`
.prune { padding: 8px; }
.buttons { display: flex; gap: 8px; flex-wrap: wrap; margin: 8px 0; }
.list { max-height: 200px; overflow: auto; display: flex; flex-wrap: wrap; gap: 4px 12px; font-family: monospace; font-size: 12px; border: 1px solid rgba(128,128,128,.4); padding: 8px; }
.msg { margin: 8px 0; font-weight: 600; }
`],
})
export class RdioScannerAdminPruneTalkgroupsComponent {
    busy = false;
    message = '';
    result: PruneResult | undefined;

    async preview(mode: 'phantom' | 'unnamed'): Promise<void> {
        this.message = '';
        this.result = await this.call(mode, false);
    }

    async confirm(): Promise<void> {
        if (!this.result) return;
        const label = this.result.mode === 'unnamed' ? 'ALL unnamed' : 'phantom';
        if (!window.confirm(`Delete ${this.result.count} ${label} talkgroup(s)? Named talkgroups and recordings are kept. Back up your database first.`)) {
            return;
        }
        const r = await this.call(this.result.mode as 'phantom' | 'unnamed', true);
        if (r) {
            this.message = `Removed ${r.count} talkgroup(s). Recordings were kept.`;
            this.result = undefined;
        }
    }

    private async call(mode: string, confirm: boolean): Promise<PruneResult | undefined> {
        this.busy = true;
        try {
            const base = window.location.href.replace(/[#?].*$/, '').replace(/\/+$/, '');
            const res = await fetch(`${base}/../api/admin/talkgroups-prune`, {
                method: 'POST',
                headers: {
                    'Authorization': window.sessionStorage?.getItem('rdio-scanner-admin-token') || '',
                    'Content-Type': 'application/json',
                },
                body: JSON.stringify({ mode, confirm }),
            });
            if (!res.ok) throw new Error(`Error ${res.status}`);
            return await res.json();
        } catch (e: any) {
            this.message = e?.message || 'Request failed';
            return undefined;
        } finally {
            this.busy = false;
        }
    }
}
