/*
 * *****************************************************************************
 * Copyright (C) 2019-2022 Chrystian Huot <chrystian.huot@saubeo.solutions>
 *
 * This program is free software: you can redistribute it and/or modify
 * it under the terms of the GNU General Public License as published by
 * the Free Software Foundation, either version 3 of the License, or
 * (at your option) any later version.
 *
 * This program is distributed in the hope that it will be useful,
 * but WITHOUT ANY WARRANTY; without even the implied warranty of
 * MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE.  See the
 * GNU General Public License for more details.
 *
 * You should have received a copy of the GNU General Public License
 * along with this program.  If not, see <http://www.gnu.org/licenses/>
 * ****************************************************************************
 */

import { Component, EventEmitter, Input, Output } from '@angular/core';
import { AbstractControl, FormArray, FormControl, FormGroup } from '@angular/forms';
import { RdioScannerAdminService, Group, Tag } from '../../../admin.service';

@Component({
    selector: 'rdio-scanner-admin-talkgroup',
    templateUrl: './talkgroup.component.html',
})
export class RdioScannerAdminTalkgroupComponent {
    @Input() form: FormGroup | undefined;

    @Output() blacklist = new EventEmitter<void>();

    @Output() remove = new EventEmitter<void>();

    leds = this.adminService.getLeds();

    get groups(): Group[] {
        return this.form?.root.get('groups')?.value as Group[];
    }

    get tags(): Tag[] {
        return this.form?.root.get('tags')?.value as Tag[];
    }

    // Per-talkgroup audio normalization: edits the "Normalize Talkgroups" option (system:talkgroup
    // entries) and saves the config, exactly like the Save button would.
    normalizeStatus = '';

    private get normalizeControl(): AbstractControl | null {
        return this.form?.root.get('options.normalizeTalkgroups') ?? null;
    }

    private get normalizeKey(): string | null {
        const system = (this.form?.parent?.parent as FormGroup | null)?.get('id')?.value;
        const talkgroup = this.form?.get('id')?.value;
        return Number.isInteger(+system) && +system > 0 && Number.isInteger(+talkgroup) && +talkgroup > 0 ? `${+system}:${+talkgroup}` : null;
    }

    private normalizeEntries(): string[] {
        return String(this.normalizeControl?.value ?? '').split(/[\s,;]+/).filter((e) => e);
    }

    get normalizeMode(): 'off' | 'norm' | 'loud' {
        const key = this.normalizeKey;
        const entry = key ? this.normalizeEntries().find((e) => e.split(':').slice(0, 2).join(':') === key) : undefined;
        return !entry ? 'off' : entry.toLowerCase().endsWith(':loud') ? 'loud' : 'norm';
    }

    // Set when no exact entry exists but the whole system is listed (e.g. 840:*).
    get normalizeViaSystem(): boolean {
        const key = this.normalizeKey;
        return !!key && this.normalizeMode === 'off' && this.normalizeEntries().some((e) => e.split(':')[0] === key.split(':')[0] && e.split(':')[1] === '*');
    }

    async setNormalize(mode: 'off' | 'norm' | 'loud'): Promise<void> {
        const key = this.normalizeKey;
        const control = this.normalizeControl;
        const root = this.form?.root;

        if (!key || !control || !root) {
            this.normalizeStatus = 'Set this talkgroup and its system Id first.';
            return;
        }

        const entries = this.normalizeEntries().filter((e) => e.split(':').slice(0, 2).join(':') !== key);
        if (mode !== 'off') entries.push(mode === 'loud' ? `${key}:loud` : key);

        control.setValue(entries.join(', '));
        control.markAsDirty();
        root.markAsDirty();

        if (root.invalid) {
            this.normalizeStatus = 'Added to the list. Fix the errors in the config, then press Save.';
            return;
        }

        root.markAsPristine();
        await this.adminService.saveConfig((root as FormGroup).getRawValue());
        this.normalizeStatus = mode === 'off' ? `Removed ${key} and saved.` : `Added ${key} and saved.`;
    }

    constructor(private adminService: RdioScannerAdminService) { }
}
