<script lang="ts">
	import { onMount } from 'svelte';
	import { Play } from 'lucide-svelte';
	import { getTasks, runTask, type Task } from '$lib/api';

	let tasks: Task[] = $state([]);
	let loading = $state(true);
	let runningTask: string | null = $state(null);
	let result: { task: string; output: string } | null = $state(null);

	onMount(async () => {
		try {
			tasks = await getTasks();
		} catch {
			// daemon might be down
		}
		loading = false;
	});

	async function execute(name: string) {
		runningTask = name;
		result = null;
		try {
			const res = await runTask(name);
			result = { task: name, output: res.output };
		} catch (e) {
			result = { task: name, output: e instanceof Error ? e.message : 'Failed' };
		}
		runningTask = null;
	}
</script>

<div class="mb-6">
	<h1 class="font-display text-3xl font-bold text-bourbon-100 lowercase">tasks</h1>
	<p class="text-bourbon-600 mt-1">Background scheduled tasks</p>
</div>

{#if loading}
	<div class="flex items-center justify-center gap-3 text-bourbon-600 py-12">
		<div class="w-4 h-4 border-2 border-bourbon-700 border-t-run-500 rounded-full animate-spin"></div>
		<span class="font-display text-xs uppercase tracking-widest">Loading</span>
	</div>
{:else}

<div class="grid grid-cols-1 gap-4">
	<div class="bg-bourbon-900 rounded-2xl border border-bourbon-800 p-6">
		{#if tasks.length === 0}
			<p class="text-bourbon-600 text-sm">No tasks registered.</p>
		{:else}
			<div class="flex flex-col gap-1.5">
				{#each tasks as task}
					<div class="flex items-center justify-between bg-bourbon-950/30 border border-bourbon-800 rounded-lg px-5 py-4">
						<div class="flex flex-col gap-0.5">
							<div class="flex items-center gap-3">
								<span class="font-semibold text-bourbon-200">{task.name}</span>
								<span class="text-xs font-medium text-cmd-400 bg-cmd-700/40 px-2.5 py-0.5 rounded-full font-mono">{task.schedule}</span>
							</div>
							{#if task.description}
								<span class="text-sm text-bourbon-500">{task.description}</span>
							{/if}
						</div>
						<button
							onclick={() => execute(task.name)}
							disabled={runningTask === task.name}
							class="btn-chiclet"
						>
							{#if runningTask === task.name}
								<div class="w-3.5 h-3.5 border-2 border-bourbon-700 border-t-cmd-300 rounded-full animate-spin"></div>
							{:else}
								<Play size={14} />
							{/if}
						</button>
					</div>
				{/each}
			</div>
		{/if}
	</div>

	{#if result}
		<div class="bg-bourbon-900 rounded-2xl border border-bourbon-800 p-6">
			<div class="flex items-center gap-2 mb-3">
				<h2 class="font-display text-xs font-bold uppercase tracking-widest text-run-500">Output</h2>
				<span class="text-xs font-medium text-cmd-400 bg-cmd-700/40 px-2.5 py-0.5 rounded-full">{result.task}</span>
			</div>
			<pre class="text-sm whitespace-pre-wrap break-words text-bourbon-300 font-mono bg-bourbon-950 rounded-lg p-4">{result.output}</pre>
		</div>
	{/if}
</div>

{/if}
