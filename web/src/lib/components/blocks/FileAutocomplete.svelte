<script lang="ts">
	import { onMount } from 'svelte';
	import { FileCode } from 'lucide-svelte';
	import { getCodeFiles } from '$lib/api';

	let {
		query,
		repoPath,
		position,
		onselect,
		oncancel
	}: {
		query: string;
		repoPath: string;
		position: { x: number; y: number };
		onselect: (file: string) => void;
		oncancel: () => void;
	} = $props();

	let results = $state<string[]>([]);
	let selectedIdx = $state(0);
	let loading = $state(false);
	let container: HTMLDivElement | undefined = $state(undefined);

	// Debounced fetch on query change
	$effect(() => {
		const q = query;
		if (q.length < 3) { results = []; return; }

		loading = true;
		const timer = setTimeout(async () => {
			try {
				results = await getCodeFiles(repoPath, q);
				selectedIdx = 0;
			} catch {
				results = [];
			}
			loading = false;
		}, 150);

		return () => clearTimeout(timer);
	});

	onMount(() => {
		function handleKeydown(e: KeyboardEvent) {
			if (e.key === 'ArrowDown') {
				e.preventDefault();
				selectedIdx = Math.min(selectedIdx + 1, results.length - 1);
			} else if (e.key === 'ArrowUp') {
				e.preventDefault();
				selectedIdx = Math.max(selectedIdx - 1, 0);
			} else if (e.key === 'Enter' && results.length > 0) {
				e.preventDefault();
				onselect(results[selectedIdx]);
			} else if (e.key === 'Escape') {
				e.preventDefault();
				oncancel();
			}
		}

		document.addEventListener('keydown', handleKeydown, true);
		return () => document.removeEventListener('keydown', handleKeydown, true);
	});

	// Scroll selected item into view
	$effect(() => {
		if (container) {
			const item = container.querySelector(`[data-idx="${selectedIdx}"]`);
			item?.scrollIntoView({ block: 'nearest' });
		}
	});

	// Highlight matched characters
	function highlight(path: string, q: string): string {
		const lPath = path.toLowerCase();
		const lQuery = q.toLowerCase();
		let qi = 0;
		let result = '';
		for (let i = 0; i < path.length; i++) {
			if (qi < lQuery.length && lPath[i] === lQuery[qi]) {
				result += `<span class="text-cmd-400">${path[i]}</span>`;
				qi++;
			} else {
				result += path[i];
			}
		}
		return result;
	}
</script>

<button type="button" class="fixed inset-0 z-40 cursor-default" onclick={oncancel} aria-label="Close autocomplete"></button>
<div
	bind:this={container}
	class="fixed z-50 bg-bourbon-900 border border-bourbon-700 rounded-lg shadow-xl py-1 min-w-[280px] max-w-[400px] max-h-[200px] overflow-y-auto"
	style="left: {position.x}px; top: {position.y}px;"
>
	{#if loading && results.length === 0}
		<div class="px-3 py-2 text-[10px] font-mono text-bourbon-600">searching...</div>
	{:else if results.length === 0}
		<div class="px-3 py-2 text-[10px] font-mono text-bourbon-600">no matches</div>
	{:else}
		{#each results as file, i}
			<button
				data-idx={i}
				class="flex items-center gap-2 w-full px-3 py-1.5 cursor-pointer transition-colors text-left
					{i === selectedIdx ? 'bg-bourbon-800 text-bourbon-100' : 'text-bourbon-400 hover:bg-bourbon-800/50'}"
				onclick={() => onselect(file)}
				onmouseenter={() => { selectedIdx = i; }}
			>
				<FileCode size={12} class="shrink-0 text-bourbon-600" />
				<span class="text-[10px] font-mono truncate">{@html highlight(file, query)}</span>
			</button>
		{/each}
	{/if}
</div>
