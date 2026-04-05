<script lang="ts">
	import { X } from 'lucide-svelte';
	import { marked } from 'marked';

	let {
		result,
		onclose
	}: {
		result: string;
		onclose: () => void;
	} = $props();

	let html = $derived(marked(result));
</script>

<!-- svelte-ignore a11y_no_static_element_interactions -->
<!-- svelte-ignore a11y_click_events_have_key_events -->
<div
	class="fixed inset-0 z-50 flex items-center justify-center bg-black/70 backdrop-blur-sm"
	onclick={onclose}
	onkeydown={(e) => { if (e.key === 'Escape') onclose(); }}
	role="dialog"
	tabindex="-1"
>
	<div
		class="bg-bourbon-900 border border-bourbon-800 rounded-2xl w-[90vw] max-w-3xl max-h-[85vh] flex flex-col overflow-hidden"
		onclick={(e) => e.stopPropagation()}
	>
		<div class="flex items-center justify-between px-6 py-4 border-b border-bourbon-800 shrink-0">
			<h2 class="font-display text-xs font-bold uppercase tracking-widest text-run-500">Review Result</h2>
			<button
				onclick={onclose}
				class="text-bourbon-600 hover:text-bourbon-300 transition-colors cursor-pointer"
			>
				<X size={18} />
			</button>
		</div>
		<div class="overflow-auto flex-1 px-6 py-4 bg-bourbon-950">
			<div class="prose prose-invert prose-sm max-w-none
				prose-headings:text-bourbon-200 prose-headings:font-display prose-headings:uppercase prose-headings:tracking-wider
				prose-p:text-bourbon-300
				prose-strong:text-bourbon-200
				prose-code:text-cmd-400 prose-code:bg-bourbon-950 prose-code:px-1 prose-code:py-0.5 prose-code:rounded
				prose-pre:bg-bourbon-950 prose-pre:border prose-pre:border-bourbon-800
				prose-a:text-cmd-400 prose-a:no-underline hover:prose-a:text-cmd-300
				prose-li:text-bourbon-300
				prose-blockquote:border-l-run-500 prose-blockquote:text-bourbon-400">
				{@html html}
			</div>
		</div>
	</div>
</div>
