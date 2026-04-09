<script lang="ts">
	import { onMount, onDestroy } from 'svelte';
	import { X, Send, Trash2 } from 'lucide-svelte';
	import {
		getRepos,
		getClaudeTaskResult,
		createDirective,
		saveDirective,
		submitDirective,
		dismissClaudeTask,
		type MonitoredRepo
	} from '$lib/api';

	let {
		initial,
		onclose,
		onsubmit
	}: {
		initial?: { repoPath?: string; content?: string; taskId?: number };
		onclose: () => void;
		onsubmit?: () => void;
	} = $props();

	let repos = $state<MonitoredRepo[]>([]);
	let taskId = $state<number | null>(null);
	let content = $state('');
	let repoPath = $state('');
	let saving = $state(false);
	let submitting = $state(false);
	let lastSavedContent = '';
	let lastSavedRepo = '';

	onMount(async () => {
		repos = await getRepos();

		content = initial?.content ?? '';
		repoPath = initial?.repoPath ?? '';

		if (!repoPath && repos.length > 0) {
			repoPath = repos[0].path;
		}

		if (initial?.taskId) {
			// Resume existing draft
			taskId = initial.taskId;
			try {
				const { result } = await getClaudeTaskResult(taskId);
				content = result || '';
			} catch { /* use initial content */ }
		} else {
			// Create new directive task
			const res = await createDirective(repoPath, content);
			taskId = res.id;
		}
		lastSavedContent = content;
		lastSavedRepo = repoPath;
	});

	// Auto-save via $effect — debounces on content/repoPath changes
	$effect(() => {
		const c = content;
		const r = repoPath;

		const timer = setTimeout(async () => {
			if (!taskId || (c === lastSavedContent && r === lastSavedRepo)) return;
			saving = true;
			await saveDirective(taskId, r, c);
			lastSavedContent = c;
			lastSavedRepo = r;
			saving = false;
		}, 1500);

		return () => clearTimeout(timer);
	});

	onDestroy(() => {
		if (taskId && (content !== lastSavedContent || repoPath !== lastSavedRepo)) {
			saveDirective(taskId, repoPath, content);
		}
	});

	async function handleSubmit() {
		if (!taskId || !content.trim() || !repoPath) return;
		submitting = true;

		await saveDirective(taskId, repoPath, content);

		try {
			await submitDirective(taskId);
			onsubmit?.();
			onclose();
		} catch {
			submitting = false;
		}
	}

	async function handleDelete() {
		if (!taskId) return;
		await dismissClaudeTask(taskId);
		onclose();
	}

	function handleKeydown(e: KeyboardEvent) {
		if (e.key === 'Enter' && (e.metaKey || e.ctrlKey)) {
			e.preventDefault();
			handleSubmit();
		}
		if (e.key === 'Escape') {
			onclose();
		}
	}

	function autofocus(node: HTMLElement) {
		requestAnimationFrame(() => node.focus());
	}
</script>

<!-- svelte-ignore a11y_click_events_have_key_events -->
<div
	class="fixed inset-0 z-50 flex items-center justify-center bg-black/70 backdrop-blur-sm"
	onmousedown={(e) => { if (e.target === e.currentTarget) onclose(); }}
	role="dialog"
	tabindex="-1"
>
	<div class="bg-bourbon-900 border border-bourbon-800 rounded-2xl w-[90vw] max-w-3xl min-h-[50vh] max-h-[85vh] flex flex-col overflow-hidden">
		<!-- Header -->
		<div class="flex items-center justify-between px-6 py-4 border-b border-bourbon-800 shrink-0">
			<div class="flex items-center gap-3">
				<h2 class="font-display text-xs font-bold uppercase tracking-widest text-cmd-400">New Directive</h2>
				{#if saving}
					<span class="text-[9px] font-mono text-bourbon-600">saving...</span>
				{/if}
			</div>
			<div class="flex items-center gap-2">
				<button
					onclick={handleDelete}
					class="text-bourbon-600 hover:text-red-400 transition-colors cursor-pointer p-1"
					title="Delete draft"
				>
					<Trash2 size={14} />
				</button>
				<button
					onclick={onclose}
					class="text-bourbon-600 hover:text-bourbon-300 transition-colors cursor-pointer"
				>
					<X size={18} />
				</button>
			</div>
		</div>

		<!-- Repo selector -->
		<div class="px-6 py-3 border-b border-bourbon-800/50 shrink-0">
			<label class="flex items-center gap-3">
				<span class="text-[10px] font-display font-bold uppercase tracking-widest text-bourbon-500">Target</span>
				<select
					bind:value={repoPath}
					class="flex-1 bg-bourbon-950 border border-bourbon-800 rounded-lg px-3 py-1.5 text-xs font-mono text-bourbon-200 focus:outline-none focus:border-cmd-500/50"
				>
					{#each repos as repo}
						<option value={repo.path}>{repo.name}</option>
					{/each}
				</select>
			</label>
		</div>

		<!-- Content editor -->
		<div class="flex-1 overflow-hidden flex flex-col bg-bourbon-950">
			<textarea
				bind:value={content}
				use:autofocus
				onkeydown={handleKeydown}
				placeholder="Describe what you want Claude to do...

You can reference code with @file:L10-L25 syntax.
Use markdown for structure."
				class="flex-1 w-full bg-transparent text-sm text-bourbon-200 px-6 py-4 resize-none focus:outline-none placeholder:text-bourbon-700 font-mono leading-relaxed select-text"
			></textarea>
		</div>

		<!-- Footer -->
		<div class="flex items-center justify-between px-6 py-3 border-t border-bourbon-800 shrink-0">
			<span class="text-[9px] text-bourbon-700">⌘+Enter to submit</span>
			{#if submitting}
				<div class="flex items-center gap-2 text-bourbon-600">
					<div class="w-3 h-3 border-2 border-bourbon-700 border-t-cmd-500 rounded-full animate-spin"></div>
					<span class="text-[10px] font-mono">launching</span>
				</div>
			{:else}
				<button
					onclick={handleSubmit}
					disabled={!content.trim() || !repoPath}
					class="flex items-center gap-1.5 text-xs text-cmd-400 hover:text-cmd-300 transition-colors cursor-pointer
						disabled:opacity-40 disabled:cursor-not-allowed"
				>
					<Send size={12} />
					Launch Claude
				</button>
			{/if}
		</div>
	</div>
</div>
