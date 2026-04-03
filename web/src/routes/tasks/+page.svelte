<script lang="ts">
	import { onMount } from 'svelte';
	import { getTasks, runTask, type Task } from '$lib/api';

	let tasks: Task[] = $state([]);
	let error: string | null = $state(null);
	let running: string | null = $state(null);
	let result: { task: string; output: string } | null = $state(null);

	onMount(async () => {
		try {
			tasks = await getTasks();
		} catch (e) {
			error = e instanceof Error ? e.message : 'Failed to load tasks';
		}
	});

	async function execute(name: string) {
		running = name;
		result = null;
		try {
			const res = await runTask(name);
			result = { task: name, output: res.output };
		} catch (e) {
			result = { task: name, output: e instanceof Error ? e.message : 'Failed' };
		} finally {
			running = null;
		}
	}
</script>

<h1 class="font-display text-3xl font-bold text-bourbon-100 lowercase mb-1">tasks</h1>
<p class="text-bourbon-600 mb-6">Manage and run scheduled tasks</p>

<hr class="border-bourbon-800 mb-8" />

<h2 class="font-display text-xs font-bold uppercase tracking-widest text-run-500 mb-4">Registered</h2>

{#if error}
	<div class="border-l-2 border-run-500 bg-bourbon-950/50 rounded-r-lg px-5 py-4">
		<h3 class="font-display text-xs font-bold uppercase tracking-widest text-run-500 mb-2">Error</h3>
		<p class="text-bourbon-400">{error}</p>
	</div>
{:else if tasks.length === 0}
	<p class="text-bourbon-600">No tasks registered yet.</p>
{:else}
	<div class="flex flex-col gap-1.5">
		{#each tasks as task}
			<div class="flex items-center justify-between bg-bourbon-950/30 border border-bourbon-800 rounded-lg px-5 py-4">
				<div class="flex flex-col gap-0.5">
					<span class="font-semibold text-bourbon-100">{task.name}</span>
					<span class="text-xs text-bourbon-600 font-mono">{task.schedule}</span>
					{#if task.description}
						<span class="text-sm text-bourbon-500">{task.description}</span>
					{/if}
				</div>
				<button
					onclick={() => execute(task.name)}
					disabled={running === task.name}
					class="px-4 py-1.5 font-display text-xs font-bold uppercase tracking-widest
						bg-cmd-500 text-cmd-300 rounded-lg
						hover:bg-cmd-400 disabled:opacity-50 disabled:cursor-not-allowed transition-colors cursor-pointer"
				>
					{running === task.name ? 'Running...' : 'Run'}
				</button>
			</div>
		{/each}
	</div>
{/if}

{#if result}
	<h2 class="font-display text-xs font-bold uppercase tracking-widest text-run-500 mt-10 mb-4">Output</h2>
	<div class="bg-bourbon-950/50 border border-bourbon-800 rounded-xl p-5">
		<div class="flex items-center gap-2 mb-3">
			<span class="text-xs font-medium text-cmd-400 bg-cmd-700/40 px-2.5 py-0.5 rounded-full">{result.task}</span>
		</div>
		<pre class="text-sm whitespace-pre-wrap break-words text-bourbon-300 font-mono">{result.output}</pre>
	</div>
{/if}
