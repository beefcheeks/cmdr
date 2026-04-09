<script lang="ts">
	import { FileCode } from 'lucide-svelte';
	import type { CodeRefBlock } from '$lib/blocks';

	let {
		block,
		onchange,
		ontrigger
	}: {
		block: CodeRefBlock;
		onchange: (ref: string) => void;
		ontrigger?: (type: string, query: string, rect: DOMRect) => void;
	} = $props();

	let localRef = $state('');
	let input: HTMLInputElement | undefined = $state(undefined);

	$effect(() => {
		localRef = block.ref;
	});

	function handleInput() {
		onchange(localRef);
		checkTrigger();
	}

	function checkTrigger() {
		if (!input || !ontrigger) return;
		const query = localRef.split(':')[0]; // strip line range for autocomplete
		if (query.length >= 3) {
			ontrigger('file', query, input.getBoundingClientRect());
		} else {
			ontrigger('dismiss', '', input.getBoundingClientRect());
		}
	}

	export function focus() {
		input?.focus();
	}

	export function setRef(ref: string) {
		localRef = ref;
		onchange(localRef);
	}
</script>

<div class="flex items-center gap-2 bg-bourbon-950 border border-bourbon-800 rounded-lg px-3 py-2">
	<span class="text-cmd-400 shrink-0"><FileCode size={14} /></span>
	<span class="text-cmd-400 text-sm font-mono shrink-0">@</span>
	<input
		bind:this={input}
		type="text"
		bind:value={localRef}
		oninput={handleInput}
		placeholder="path/to/file:L10-L25"
		class="flex-1 bg-transparent text-sm text-cmd-300 font-mono focus:outline-none placeholder:text-bourbon-700"
	/>
</div>
