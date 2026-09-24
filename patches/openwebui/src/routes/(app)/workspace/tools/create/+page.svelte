<script>
	import { onMount } from 'svelte';
	import { goto } from '$app/navigation';

	onMount(() => {
		goto('/workspace/tools?action=add-mcp', { replaceState: true });
	});
</script>

<div class="flex items-center justify-center h-full">
	<div class="text-sm text-gray-500 animate-pulse">Redirecting to MCP Tools...</div>
</div>
