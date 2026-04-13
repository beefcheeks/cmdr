import { readable, derived } from 'svelte/store';

// Global reactive clock — ticks every 60s so relative timestamps stay fresh.
// Uses recursive setTimeout to avoid setInterval drift under CPU pressure.
export const now = readable(new Date(), (set) => {
	let timeout: ReturnType<typeof setTimeout>;
	function tick() {
		set(new Date());
		timeout = setTimeout(tick, 60_000);
	}
	timeout = setTimeout(tick, 60_000);
	return () => clearTimeout(timeout);
});

// Reactive timeAgo — returns a store-derived function that recalculates every tick.
export const timeAgo = derived(now, ($now) => (dateStr: string): string => {
	const date = new Date(dateStr);
	const seconds = Math.floor(($now.getTime() - date.getTime()) / 1000);
	if (seconds < 60) return 'just now';
	const minutes = Math.floor(seconds / 60);
	if (minutes < 60) return `${minutes}m ago`;
	const hours = Math.floor(minutes / 60);
	if (hours < 24) return `${hours}h ago`;
	const days = Math.floor(hours / 24);
	return `${days}d ago`;
});
