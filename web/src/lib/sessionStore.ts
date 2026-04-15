import { writable } from 'svelte/store';
import { killTmuxSession, type TmuxSession, type ClaudeSession } from '$lib/api';
import { events } from '$lib/events';

// --- Core state ---

export const sessions = writable<TmuxSession[]>([]);
export const claudeSessions = writable<ClaudeSession[]>([]);
export const sessionsLoaded = writable(false);

// --- Actions ---

export async function killSession(name: string) {
	await killTmuxSession(name);
	sessions.update((s) => s.filter((sess) => sess.name !== name));
}

export function markAttached(name: string) {
	sessions.update((s) => s.map((sess) => ({ ...sess, attached: sess.name === name })));
}

// --- SSE sync ---

let initialized = false;

export function initSessionStore() {
	if (initialized) return;
	initialized = true;

	events.on('tmux:sessions', (data) => {
		sessions.set(data);
		sessionsLoaded.set(true);
	});

	events.on('claude:sessions', (data: ClaudeSession[]) => {
		claudeSessions.set(data);
		// Signal native app (cmdr.app) about claude activity for menubar indicator
		if ((window as any).webkit?.messageHandlers?.activity) {
			const hasActive = data.some((s: ClaudeSession) => s.status === 'working');
			(window as any).webkit.messageHandlers.activity.postMessage({ active: hasActive });
		}
	});
}
