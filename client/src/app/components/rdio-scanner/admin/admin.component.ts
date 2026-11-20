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

import { Component, OnDestroy, ViewEncapsulation } from '@angular/core';
import { AdminEvent, RdioScannerAdminService, Group, Tag, UpdateInfo } from './admin.service';

@Component({
    encapsulation: ViewEncapsulation.None,
    selector: 'rdio-scanner-admin',
    styleUrls: ['./admin.component.scss'],
    templateUrl: './admin.component.html',
})
export class RdioScannerAdminComponent implements OnDestroy {
    authenticated = this.adminService.authenticated;

    groups: Group[] = [];

    tags: Tag[] = [];

    // Set only when a newer release than the running exe is published on GitHub.
    updateAvailable: UpdateInfo | undefined;

    // Latest update-check answer, for the "About this build" line.
    buildInfo: UpdateInfo | undefined;

    private eventSubscription = this.adminService.event.subscribe(async (event: AdminEvent) => {
        if ('authenticated' in event) {
            this.authenticated = event.authenticated || false;

            if (this.authenticated) {
                this.checkForUpdate();
            }
        }
    });

    constructor(private adminService: RdioScannerAdminService) {
        if (this.authenticated) {
            this.checkForUpdate();
        }
    }

    private async checkForUpdate(): Promise<void> {
        const info = await this.adminService.checkForUpdate();

        this.buildInfo = info;
        this.updateAvailable = info?.status === 'available' ? info : undefined;
    }

    ngOnDestroy(): void {
        this.eventSubscription.unsubscribe();
    }

    // One line describing the running build.
    get buildLine(): string {
        const i = this.buildInfo;
        if (!i) return '';
        const base = i.version ? `Rdio Scanner ${i.version}` : 'Rdio Scanner';
        switch (i.status) {
            case 'current': return `${base} · ${i.current?.name || i.current?.tag} · up to date`;
            case 'available': return `${base} · ${i.current?.name || i.current?.tag}`;
            case 'unknown': return `${base} · local build (not a published release)`;
            default: return `${base} · could not check for updates`;
        }
    }

    async logout(): Promise<void> {
        await this.adminService.logout();
    }
}
