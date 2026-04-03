<script lang="ts">
	import { onMount } from 'svelte';
	import { FolderGit2, Plus, Trash2, RefreshCw } from 'lucide-svelte';
	import {
		getRepos,
		discoverRepos,
		addRepo,
		removeRepo,
		syncRepos,
		type MonitoredRepo,
		type DiscoveredRepo
	} from '$lib/api';

	let repos: MonitoredRepo[] = $state([]);
	let discovered: DiscoveredRepo[] = $state([]);
	let loading = $state(true);
	let syncing = $state(false);
	let showAddRepo = $state(false);
	let discovering = $state(false);
	let repoSearch = $state('');

	onMount(async () => {
		try {
			repos = await getRepos();
		} catch {
			// daemon might be down
		}
		loading = false;
	});

	async function openAddRepo() {
		showAddRepo = true;
		discovering = true;
		repoSearch = '';
		try {
			discovered = await discoverRepos();
		} catch {
			discovered = [];
		}
		discovering = false;
	}

	async function handleAddRepo(repo: DiscoveredRepo) {
		await addRepo(repo);
		discovered = discovered.filter(r => r.path !== repo.path);
		repos = await getRepos();
	}

	async function handleRemoveRepo(id: number) {
		await removeRepo(id);
		repos = await getRepos();
	}

	async function handleSync() {
		syncing = true;
		await syncRepos();
		setTimeout(async () => {
			repos = await getRepos();
			syncing = false;
		}, 3000);
	}

	let filteredDiscovered = $derived(
		repoSearch
			? discovered.filter(r => r.name.toLowerCase().includes(repoSearch.toLowerCase()))
			: discovered
	);

	function timeAgo(dateStr: string): string {
		const date = new Date(dateStr);
		const now = new Date();
		const seconds = Math.floor((now.getTime() - date.getTime()) / 1000);
		if (seconds < 60) return 'just now';
		const minutes = Math.floor(seconds / 60);
		if (minutes < 60) return `${minutes}m ago`;
		const hours = Math.floor(minutes / 60);
		if (hours < 24) return `${hours}h ago`;
		const days = Math.floor(hours / 24);
		return `${days}d ago`;
	}

	function shortenPath(path: string): string {
		return path.replace(/^\/Users\/[^/]+/, '~');
	}
</script>

<div class="mb-6">
	<h1 class="font-display text-3xl font-bold text-bourbon-100 lowercase">settings</h1>
	<p class="text-bourbon-600 mt-1">Configure monitored repos</p>
</div>

{#if loading}
	<div class="flex items-center justify-center gap-3 text-bourbon-600 py-12">
		<div class="w-4 h-4 border-2 border-bourbon-700 border-t-run-500 rounded-full animate-spin"></div>
		<span class="font-display text-xs uppercase tracking-widest">Loading</span>
	</div>
{:else}

<!-- Monitored Repos -->
<div class="bg-bourbon-900 rounded-2xl border border-bourbon-800 p-6">
	<div class="flex items-center justify-between mb-4">
		<h2 class="font-display text-xs font-bold uppercase tracking-widest text-run-500">Monitored Repos</h2>
		<div class="flex items-center gap-2">
			<button
				onclick={handleSync}
				class="flex items-center gap-1.5 px-3 py-1.5 text-xs font-display font-bold uppercase tracking-widest
					text-bourbon-500 hover:text-bourbon-300 transition-colors cursor-pointer"
				disabled={syncing}
			>
				<RefreshCw size={12} class={syncing ? 'animate-spin' : ''} />
				Sync now
			</button>
			<button
				onclick={openAddRepo}
				class="btn-chiclet"
			>
				<Plus size={14} />
			</button>
		</div>
	</div>

	{#if repos.length === 0}
		<p class="text-bourbon-600 text-sm">No repos monitored yet. Click + to add one.</p>
	{:else}
		<div class="flex flex-col gap-1.5">
			{#each repos as repo}
				<div class="group flex items-center justify-between bg-bourbon-950/30 border border-bourbon-800 rounded-lg px-5 py-3.5">
					<div class="flex items-center gap-3">
						<FolderGit2 size={14} class="text-cmd-400" />
						<span class="text-bourbon-200">{repo.name}</span>
						<span class="text-xs text-bourbon-600 font-mono">{shortenPath(repo.path)}</span>
						{#if repo.lastSyncedAt}
							<span class="text-xs text-bourbon-600">&middot; synced {timeAgo(repo.lastSyncedAt)}</span>
						{:else}
							<span class="text-xs text-run-500">syncing...</span>
						{/if}
					</div>
					<button
						onclick={() => handleRemoveRepo(repo.id)}
						class="opacity-0 group-hover:opacity-100 text-bourbon-700 hover:text-red-400 transition-all cursor-pointer"
					>
						<Trash2 size={14} />
					</button>
				</div>
			{/each}
		</div>
	{/if}

	<!-- Add Repo Panel -->
	{#if showAddRepo}
		<div class="mt-4 bg-bourbon-950/50 border border-bourbon-800 rounded-lg p-4">
			<div class="flex items-center justify-between mb-3">
				<h3 class="font-display text-xs font-bold uppercase tracking-widest text-cmd-400">Add Repository</h3>
				<button
					onclick={() => { showAddRepo = false; repoSearch = ''; }}
					class="text-bourbon-600 hover:text-bourbon-400 text-xs cursor-pointer"
				>
					cancel
				</button>
			</div>

			{#if !discovering}
				<input
					type="text"
					placeholder="Filter repos..."
					bind:value={repoSearch}
					class="w-full bg-bourbon-900 border border-bourbon-700 rounded-lg px-3 py-2 text-sm text-bourbon-200
						placeholder:text-bourbon-600 focus:outline-none focus:border-cmd-500 mb-3"
				/>
			{/if}

			<div class="max-h-64 overflow-y-auto flex flex-col gap-1">
				{#if discovering}
					<div class="flex items-center justify-center gap-2 py-4 text-bourbon-600">
						<div class="w-3 h-3 border-2 border-bourbon-700 border-t-cmd-500 rounded-full animate-spin"></div>
						<span class="text-xs">Scanning for repos...</span>
					</div>
				{:else}
					{#each filteredDiscovered as repo}
						<button
							onclick={() => handleAddRepo(repo)}
							class="flex items-center justify-between px-3 py-2 rounded-md text-left
								text-bourbon-300 hover:bg-bourbon-800/50 transition-colors cursor-pointer"
						>
							<div class="flex items-center gap-2">
								<FolderGit2 size={12} class="text-bourbon-600" />
								<span class="text-sm">{repo.name}</span>
								<span class="text-xs text-bourbon-600 font-mono">{shortenPath(repo.path)}</span>
							</div>
							<Plus size={14} class="text-bourbon-600" />
						</button>
					{:else}
						<p class="text-bourbon-600 text-sm px-3 py-2">
							{repoSearch ? 'No matching repos' : 'No new repos found'}
						</p>
					{/each}
				{/if}
			</div>
		</div>
	{/if}
</div>

{/if}
