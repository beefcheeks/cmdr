<script lang="ts">
	import { onMount, onDestroy } from 'svelte';
	import { ArrowRightLeft, Sparkles, X } from 'lucide-svelte';
	import {
		getTasks,
		killTmuxSession,
		switchTmuxSession,
		type Task,
		type TmuxSession,
		type ClaudeSession
	} from '$lib/api';
	import { events } from '$lib/events';

	let tasks: Task[] = $state([]);
	let sessions: TmuxSession[] = $state([]);
	let claudeSessions: ClaudeSession[] = $state([]);
	let error: string | null = $state(null);
	let loaded = $state({ tasks: false, tmux: false, claude: false });
	let ready = $derived(loaded.tasks && loaded.tmux && loaded.claude);

	const now = new Date();
	const hour = now.getHours();
	const greeting = hour < 12 ? 'good morning' : hour < 17 ? 'good afternoon' : 'good evening';
	const dateStr = now.toLocaleDateString('en-US', {
		weekday: 'long',
		month: 'long',
		day: 'numeric'
	});

	onMount(async () => {
		// Fallback: show content after 3s even if not all sources responded
		setTimeout(() => {
			loaded = { tasks: true, tmux: true, claude: true };
		}, 3000);

		try {
			tasks = await getTasks();
			loaded.tasks = true;
		} catch (e) {
			error = e instanceof Error ? e.message : 'Failed to connect to daemon';
			loaded = { tasks: true, tmux: true, claude: true };
		}
	});

	const unsubTmux = events.on('tmux:sessions', (data) => {
		sessions = data;
		loaded.tmux = true;
	});

	const unsubClaude = events.on('claude:sessions', (data) => {
		claudeSessions = data;
		loaded.claude = true;
	});

	onDestroy(() => {
		unsubTmux();
		unsubClaude();
	});

	let holdingKill: string | null = $state(null);
	let holdProgress: number = $state(0);
	let holdRaf: number | null = null;
	let holdStart: number = 0;
	let killedSession: string | null = $state(null);

	const HOLD_DURATION = 800; // ms to fill

	function startHoldKill(name: string) {
		holdingKill = name;
		holdProgress = 0;
		holdStart = 0;

		function tick(timestamp: number) {
			if (!holdStart) holdStart = timestamp;
			holdProgress = Math.min((timestamp - holdStart) / HOLD_DURATION, 1);
			if (holdProgress >= 1) {
				completeKill(name);
				return;
			}
			holdRaf = requestAnimationFrame(tick);
		}

		holdRaf = requestAnimationFrame(tick);
	}

	function cancelHoldKill() {
		if (holdRaf) cancelAnimationFrame(holdRaf);
		holdRaf = null;
		holdingKill = null;
		holdProgress = 0;
	}

	async function completeKill(name: string) {
		if (holdRaf) cancelAnimationFrame(holdRaf);
		holdRaf = null;
		holdingKill = null;
		holdProgress = 0;
		killedSession = name;
		await killTmuxSession(name);
		// Keep "killed" state visible so user can lift finger safely
		setTimeout(() => {
			sessions = sessions.filter((s) => s.name !== name);
			killedSession = null;
		}, 3000);
	}

	async function switchTo(name: string) {
		sessions = sessions.map((s) => ({ ...s, attached: s.name === name }));
		await switchTmuxSession(name);
	}

	function shortenPath(path: string): string {
		return path.replace(/^\/Users\/[^/]+/, '~');
	}

	// Normalize name the same way tmux-sessionizer does: . - space → _
	function normalize(name: string): string {
		return name.replace(/[.\- ]/g, '_');
	}

	// Map of tmux session name → matched Claude session, re-derived on every update
	let claudeBySession = $derived(
		new Map(
			claudeSessions
				.filter((c) => sessions.some((s) => s.name === normalize(c.project)))
				.map((c) => [normalize(c.project), c])
		)
	);

	// Claude instances not matched to any tmux session
	let unmatchedClaude = $derived(
		claudeSessions.filter((c) => !sessions.some((s) => s.name === normalize(c.project)))
	);
</script>

<!-- Greeting -->
<div class="mb-6">
	<h1 class="font-display text-3xl font-bold text-bourbon-100 lowercase">{greeting}, mike</h1>
	<p class="text-bourbon-600 mt-1">
		{dateStr}
		&middot; {sessions.length} session{sessions.length !== 1 ? 's' : ''}
		&middot; {claudeSessions.length} claude instance{claudeSessions.length !== 1 ? 's' : ''}
	</p>
</div>

<hr class="border-bourbon-800 mb-8" />

{#if !ready}
	<div class="flex items-center justify-center gap-3 text-bourbon-600 py-12">
		<div class="w-4 h-4 border-2 border-bourbon-700 border-t-run-500 rounded-full animate-spin"></div>
		<span class="font-display text-xs uppercase tracking-widest">Loading</span>
	</div>
{:else}

<!-- Tmux Sessions -->
{#if sessions.length > 0}
	<h2 class="font-display text-xs font-bold uppercase tracking-widest text-run-500 mb-4">Sessions</h2>

	<div class="flex flex-col gap-1.5 mb-10">
		{#each sessions as session}
			{@const claude = claudeBySession.get(session.name)}
			{#if killedSession === session.name}
				<div class="flex items-center justify-center border border-red-900/30 rounded-lg px-5 py-3.5 text-red-400
					animate-fade-out">
					<span class="font-display text-xs font-bold uppercase tracking-widest">killed {session.name}</span>
				</div>
			{:else}
			<div class="group flex items-center gap-4 {session.attached ? 'bg-bourbon-800/40' : 'bg-bourbon-950/30'} border border-bourbon-800 rounded-lg px-5 py-3.5">
				<div class="flex-1 min-w-0">
					<div class="flex items-center gap-2 mb-2">
						<div
							class="w-2 h-2 rounded-full {session.attached
								? 'bg-run-500'
								: 'bg-bourbon-700'}"
						></div>
						<span class="font-semibold text-bourbon-100">{session.name}</span>
						<span class="text-xs text-bourbon-600">{session.windows.length} window{session.windows.length !== 1 ? 's' : ''}</span>
						{#if session.attached}
							<span class="text-xs font-medium text-run-500 bg-run-700/30 px-2.5 py-0.5 rounded-full">attached</span>
						{/if}
						{#if claude}
							{@const statusStyle = {
								working: 'text-green-400 bg-green-900/30',
								waiting: 'text-run-400 bg-run-700/30 animate-pulse',
								idle: 'text-bourbon-500 bg-bourbon-800/30',
								unknown: 'text-cmd-400 bg-cmd-700/30'
							}[claude.status]}
							{@const statusLabel = {
								working: 'claude · working',
								waiting: 'claude · waiting',
								idle: `claude · idle · ${claude.uptime}`,
								unknown: `claude · ${claude.uptime}`
							}[claude.status]}
							<span class="flex items-center gap-1 text-xs font-medium px-2.5 py-0.5 rounded-full {statusStyle}">
								<Sparkles size={10} />
								{statusLabel}
							</span>
						{/if}
					</div>
					<div class="flex flex-col gap-1 ml-4">
						{#each session.windows as window}
							{#each window.panes as pane}
								<div class="flex items-center gap-3 text-sm">
									<span class="text-bourbon-600 font-mono text-xs">{pane.command}</span>
									<span class="text-bourbon-500 font-mono text-xs">{shortenPath(pane.cwd)}</span>
								</div>
							{/each}
						{/each}
					</div>
				</div>
				<div class="flex items-center gap-2 opacity-0 group-hover:opacity-100 transition-opacity">
					{#if !session.attached}
						<button
							onclick={() => switchTo(session.name)}
							class="btn-chiclet"
						>
							<ArrowRightLeft size={18} />
						</button>
					{/if}
					<button
						onmousedown={() => startHoldKill(session.name)}
						onmouseup={cancelHoldKill}
						onmouseleave={cancelHoldKill}
						class="btn-chiclet-danger relative overflow-hidden"
					>
						{#if holdingKill === session.name}
							<div
								class="absolute inset-x-0 bottom-0 bg-red-500/40 transition-none"
								style="height: {holdProgress * 100}%"
							></div>
						{/if}
						<X size={18} class="relative z-10" />
					</button>
				</div>
			</div>
			{/if}
		{/each}
	</div>
{/if}

<!-- Standalone Claude instances (not matched to a tmux session) -->
{#if unmatchedClaude.length > 0}
	<h2 class="font-display text-xs font-bold uppercase tracking-widest text-run-500 mb-4">Claude Instances</h2>

	<div class="flex flex-col gap-1.5 mb-10">
		{#each unmatchedClaude as instance}
			<div class="flex items-center gap-3 bg-bourbon-950/30 border border-bourbon-800 rounded-lg px-5 py-3.5">
				<span class="text-cmd-400"><Sparkles size={14} /></span>
				<span class="font-semibold text-bourbon-100">{instance.project}</span>
				<span class="text-xs text-bourbon-600 font-mono">{shortenPath(instance.cwd)}</span>
				<span class="text-xs text-bourbon-600">&middot; {instance.uptime}</span>
				<span class="text-xs text-bourbon-600">&middot; pid {instance.pid}</span>
			</div>
		{/each}
	</div>
{/if}

<!-- Scheduled Tasks -->
<h2 class="font-display text-xs font-bold uppercase tracking-widest text-run-500 mb-4">Scheduled Tasks</h2>

{#if tasks.length === 0}
	<p class="text-bourbon-600">No tasks registered yet.</p>
{:else}
	<div class="flex flex-col gap-1.5 mb-10">
		{#each tasks as task}
			<div class="flex items-center justify-between bg-bourbon-950/30 border border-bourbon-800 rounded-lg px-5 py-3.5">
				<div class="flex items-center gap-3">
					<div class="w-4 h-4 rounded border-2 border-bourbon-700 shrink-0"></div>
					<div>
						<span class="text-bourbon-200">{task.name}</span>
						{#if task.description}
							<span class="text-bourbon-600 ml-2 text-sm">{task.description}</span>
						{/if}
					</div>
				</div>
				<span class="text-xs font-medium text-cmd-400 bg-cmd-700/40 px-2.5 py-0.5 rounded-full"
					>{task.schedule}</span
				>
			</div>
		{/each}
	</div>
{/if}

<!-- Note -->
{#if error}
	<div class="border-l-2 border-run-500 bg-bourbon-950/50 rounded-r-lg px-5 py-4">
		<h3 class="font-display text-xs font-bold uppercase tracking-widest text-run-500 mb-2">Note</h3>
		<p class="text-bourbon-400">{error}</p>
	</div>
{/if}

{/if}
