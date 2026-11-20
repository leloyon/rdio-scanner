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

import { Component, OnDestroy, OnInit } from '@angular/core';
import { AdminEvent, Config, RdioScannerAdminService } from '../admin.service';

interface Todo {
    level: 'info' | 'warn';
    message: String;
}

@Component({
    selector: 'rdio-scanner-admin-todos',
    styleUrls: ['./todos.component.scss'],
    templateUrl: './todos.component.html',
})
export class RdioScannerAdminTodosComponent implements OnDestroy, OnInit {
    todos: Todo[] = [];

    private config: Config | undefined;

    private eventSubscription = this.adminService.event.subscribe(async (event: AdminEvent) => {
        if ('config' in event) {
            this.config = event.config;
        }

        if ('ffmpeg' in event) {
            this.ffmpeg = event.ffmpeg;
        }

        if ('passwordNeedChange' in event) {
            this.passwordNeedChange = event.passwordNeedChange || false;
        }

        this.rebuildTodos();
    });

    private ffmpeg = this.adminService.ffmpeg;

    private passwordNeedChange = this.adminService.passwordNeedChange;

    constructor(private adminService: RdioScannerAdminService) { }

    ngOnDestroy(): void {
        this.eventSubscription.unsubscribe();
    }

    ngOnInit(): void {
        this.adminService.getConfig().then((config) => {
            this.config = config;

            this.rebuildTodos();
        });
    }

    private rebuildTodos(): void {
        const todos: Todo[] = [];

        if (this.passwordNeedChange) {
            todos.push({
                level: 'warn',
                message: 'You are using the default admin password, please change it from the tools / admin password menu.'
            });
        }

        if (!this.config?.systems?.length) {
            todos.push({
                level: 'info',
                message: 'No systems defined. You can define one from the systems menu, or import one from a CSV file from the tools menu, or turn on the global auto populate option from the options menu.',
            });
        }

        if (!this.config?.apiKeys?.length && !this.config?.dirWatch?.length) {
            todos.push({
                level: 'info',
                message: 'No apikeys or dirwatch defined. Please set at least one to allow ingesting audio files.',
            });
        }

        // Normalization is wanted when the global choice normalizes, or talkgroups are listed.
        const options = this.config?.options;
        const wantsConversion = (options?.audioConversion ?? 0) > 0 || !!options?.normalizeTalkgroups?.trim();
        const wantsNormalization = (options?.audioConversion ?? 0) >= 2 || !!options?.normalizeTalkgroups?.trim();

        if (this.ffmpeg && !this.ffmpeg.available && wantsConversion) {
            todos.push({
                level: 'warn',
                message: 'FFmpeg was not found, so audio conversion and normalization are turned off. Install FFmpeg 4.3 or newer, make sure it is on the PATH, then restart Rdio Scanner.',
            });

        } else if (this.ffmpeg?.available && !this.ffmpeg.version43 && wantsNormalization) {
            todos.push({
                level: 'warn',
                message: 'FFmpeg was found but is older than 4.3 or its version text is not recognized (it must look like "7.1.1", with three numbers), so audio normalization is skipped. Install a release build of FFmpeg 4.3 or newer, then restart Rdio Scanner.',
            });
        }

        this.todos = todos;
    }
}
