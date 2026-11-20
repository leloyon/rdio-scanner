/*
 * Lock-screen controls (Media Session): shows the current call and play/pause, skip and replay.
 */

export interface BackgroundAudioHandlers {
    pause: () => void;
    play: () => void;
    next: () => void;
    previous: () => void;
}

export class BackgroundAudio {
    private active = false;
    private handlers: BackgroundAudioHandlers | undefined;

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

    start(): void {
        this.active = true;
        this.setState(false);
    }

    stop(): void {
        this.active = false;
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
    }

    setMetadata(title: string, artist: string, album: string): void {
        try {
            const ms = (navigator as any).mediaSession;
            const MM = (window as any).MediaMetadata;
            if (ms && MM) ms.metadata = new MM({ title, artist, album });
        } catch { }
    }
}

export const backgroundAudio = new BackgroundAudio();
