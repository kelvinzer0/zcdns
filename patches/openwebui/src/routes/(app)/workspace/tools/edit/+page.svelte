<script>
	import { onMount } from 'svelte';
	import { goto } from '$app/navigation';
	import { page } from '$app/stores';

	onMount(() => {
		const id = $page.url.searchParams.get('id') || '';
		goto(`/workspace/tools/create?id=${encodeURIComponent(id)}`, { replaceState: true });
	});
</script>

<div class="flex items-center justify-center h-full">
	<div class="text-sm text-gray-500 animate-pulse">Loading Tool Server...</div>
</div>
