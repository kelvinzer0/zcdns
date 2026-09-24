<script lang="ts">
	import { onMount, getContext } from 'svelte';
	import { goto } from '$app/navigation';
	import { page } from '$app/stores';
	import { toast } from 'svelte-sonner';
	import { v4 as uuidv4 } from 'uuid';
	import {
		getToolServerConnections,
		setToolServerConnections,
		verifyToolServerConnection
	} from '$lib/apis/configs';
	import { deleteToolById } from '$lib/apis/tools';

	import Switch from '$lib/components/common/Switch.svelte';
	import Spinner from '$lib/components/common/Spinner.svelte';
	import Badge from '$lib/components/common/Badge.svelte';
	import ChevronLeft from '$lib/components/icons/ChevronLeft.svelte';
	import ChevronDown from '$lib/components/icons/ChevronDown.svelte';
	import ChevronUp from '$lib/components/icons/ChevronUp.svelte';
	import WrenchAlt from '$lib/components/icons/WrenchAlt.svelte';
	import GarbageBin from '$lib/components/icons/GarbageBin.svelte';
	import Search from '$lib/components/icons/Search.svelte';

	const i18n = getContext('i18n');

	let loading = true;
	let verifying = false;
	let saving = false;
	let deleting = false;

	let editMode = false;
	let serverId = '';

	let type = 'mcp'; // 'mcp' or 'openapi'
	let url = '';
	let name = '';
	let key = '';
	let authType = 'bearer';
	let enable = true;

	let verified = false;
	let latencyMs: number | null = null;
	let verifyError = '';
	let discoveredTools: any[] = [];
	let disabledTools: string[] = [];
	let toolSearchQuery = '';
	let expandedToolIdx: number | null = null;

	const DEMO_URL = 'https://public-mcp-bridge.warunglakku.com/mcp?room=bc61a142';
	const ARIA_URL = 'https://github.com/kelvinzer0/aria-page-agent';

	const autoSuggestName = () => {
		if (name.trim() !== '') return;
		try {
			if (url.includes('warunglakku.com')) {
				name = 'Public MCP Bridge';
			} else if (url.includes('aria')) {
				name = 'Aria Browser Agent';
			} else {
				const u = new URL(url);
				const room = u.searchParams.get('room');
				if (room) {
					name = `MCP Bridge (${room})`;
				} else {
					name = u.hostname.replace('www.', '');
				}
			}
		} catch (e) {
			if (url.length > 5) {
				name = 'MCP Server';
			}
		}
	};

	const fillDemoUrl = () => {
		url = DEMO_URL;
		name = 'Public MCP Bridge Demo';
		type = 'mcp';
		verified = false;
		verifyError = '';
		discoveredTools = [];
		disabledTools = [];
	};

	const isToolEnabled = (toolName: string) => {
		return !disabledTools.includes(toolName);
	};

	const toggleToolStatus = (toolName: string, isEnabled: boolean) => {
		if (isEnabled) {
			disabledTools = disabledTools.filter((t) => t !== toolName);
		} else {
			if (!disabledTools.includes(toolName)) {
				disabledTools = [...disabledTools, toolName];
			}
		}
	};

	const enableAllTools = () => {
		disabledTools = [];
		toast.success('Semua tools diaktifkan');
	};

	const disableAllTools = () => {
		disabledTools = discoveredTools.map((t) => t.name).filter(Boolean);
		toast.info('Semua tools dinonaktifkan');
	};

	const handleTestConnection = async () => {
		const targetUrl = url.trim();
		if (!targetUrl) {
			toast.error('Silakan masukkan Server URL terlebih dahulu');
			return false;
		}

		verifying = true;
		verifyError = '';
		const startTime = performance.now();

		try {
			const res = await verifyToolServerConnection(localStorage.token, {
				url: targetUrl,
				type,
				auth_type: authType,
				key: key.trim()
			});

			latencyMs = Math.round(performance.now() - startTime);

			if (res && res.status) {
				verified = true;
				discoveredTools = res.specs || [];
				autoSuggestName();
				toast.success(`Koneksi berhasil terhubung! (${discoveredTools.length} tool ditemukan)`);
				return true;
			} else {
				throw new Error(res?.detail || 'Gagal memverifikasi koneksi');
			}
		} catch (err: any) {
			verified = false;
			verifyError = err.message || 'Gagal terhubung ke MCP server. Pastikan URL dan endpoint SSE aktif.';
			toast.error(`Koneksi gagal: ${verifyError}`);
			return false;
		} finally {
			verifying = false;
		}
	};

	const handleSave = async () => {
		const targetUrl = url.trim();
		if (!targetUrl) {
			toast.error('Server URL tidak boleh kosong');
			return;
		}

		if (!verified) {
			const ok = await handleTestConnection();
			if (!ok) return;
		}

		if (!name.trim()) {
			autoSuggestName();
			if (!name.trim()) {
				name = 'MCP Server';
			}
		}

		saving = true;
		try {
			const res = await getToolServerConnections(localStorage.token).catch(() => null);
			let connections: any[] = res?.TOOL_SERVER_CONNECTIONS || [];

			const targetId = serverId || `mcp_${uuidv4().slice(0, 8)}`;
			const enabledToolNames = discoveredTools
				.map((t) => t.name)
				.filter((n) => n && !disabledTools.includes(n));

			const newServer = {
				id: targetId,
				name: name.trim(),
				type,
				url: targetUrl,
				auth_type: authType,
				key: key.trim(),
				config: {
					enable,
					disabled_tools: disabledTools,
					function_name_filter_list: enabledToolNames.join(',')
				},
				info: {
					id: targetId,
					name: name.trim(),
					description: `MCP Server (${type}) via ${targetUrl}`,
					specs: discoveredTools
				}
			};

			const existingIdx = connections.findIndex((s) => s.id === targetId || s.info?.id === targetId);
			if (existingIdx >= 0) {
				connections[existingIdx] = newServer;
			} else {
				connections.push(newServer);
			}

			const saveRes = await setToolServerConnections(localStorage.token, {
				TOOL_SERVER_CONNECTIONS: connections
			}).catch((e) => {
				throw new Error(e?.message || e);
			});

			if (saveRes) {
				toast.success(editMode ? 'Server berhasil diperbarui' : 'MCP Server berhasil ditambahkan!');
				goto('/workspace/tools');
			}
		} catch (err: any) {
			toast.error(`Gagal menyimpan server: ${err.message || err}`);
		} finally {
			saving = false;
		}
	};

	const handleDelete = async () => {
		if (!serverId) return;
		if (!confirm('Apakah Anda yakin ingin menghapus MCP server ini? Semua konfigurasi dan tools terkait akan dihapus.')) {
			return;
		}

		deleting = true;
		try {
			const res = await getToolServerConnections(localStorage.token).catch(() => null);
			let connections: any[] = res?.TOOL_SERVER_CONNECTIONS || [];
			connections = connections.filter((s) => s.id !== serverId && s.info?.id !== serverId);

			await setToolServerConnections(localStorage.token, {
				TOOL_SERVER_CONNECTIONS: connections
			});

			await deleteToolById(localStorage.token, 'server:mcp:' + serverId).catch(() => null);
			await deleteToolById(localStorage.token, serverId).catch(() => null);

			toast.success('MCP Server berhasil dihapus');
			goto('/workspace/tools');
		} catch (err: any) {
			toast.error(`Gagal menghapus server: ${err.message || err}`);
		} finally {
			deleting = false;
		}
	};

	$: filteredTools = discoveredTools.filter((t) => {
		if (!toolSearchQuery.trim()) return true;
		const q = toolSearchQuery.toLowerCase();
		return (
			(t.name && t.name.toLowerCase().includes(q)) ||
			(t.description && t.description.toLowerCase().includes(q))
		);
	});

	$: activeToolsCount = discoveredTools.filter((t) => !disabledTools.includes(t.name)).length;

	onMount(async () => {
		const targetId = $page.url.searchParams.get('id');
		const isClone = $page.url.searchParams.get('clone') === 'true';
		if (targetId) {
			const res = await getToolServerConnections(localStorage.token).catch(() => null);
			const connections: any[] = res?.TOOL_SERVER_CONNECTIONS || [];
			const found = connections.find((s) => s.id === targetId || s.info?.id === targetId);

			if (found) {
				if (isClone) {
					editMode = false;
					serverId = '';
					name = `${found.name || found.info?.name || 'MCP'} (Copy)`;
				} else {
					editMode = true;
					serverId = targetId;
					name = found.name || found.info?.name || '';
				}
				url = found.url || '';
				type = found.type || 'mcp';
				key = found.key || '';
				authType = found.auth_type || 'bearer';
				enable = found.config?.enable ?? true;

				// Load disabled tools list
				if (Array.isArray(found.config?.disabled_tools)) {
					disabledTools = found.config.disabled_tools;
				} else if (typeof found.config?.function_name_filter_list === 'string' && found.config.function_name_filter_list) {
					const allowed = found.config.function_name_filter_list.split(',').map((s: string) => s.trim());
					if (Array.isArray(found.info?.specs)) {
						disabledTools = found.info.specs
							.map((s: any) => s.name)
							.filter((n: string) => n && !allowed.includes(n));
					}
				}

				// If cached specs exist, load immediately
				if (Array.isArray(found.info?.specs) && found.info.specs.length > 0) {
					discoveredTools = found.info.specs;
					verified = true;
				}

				// Auto test connection
				if (url) {
					handleTestConnection();
				}
			}
		}
		loading = false;
	});
</script>

<svelte:head>
	<title>{editMode ? 'Edit Tool Server' : 'Add MCP Server'} / {$i18n.t('Tools')}</title>
</svelte:head>

<div class="flex flex-col w-full h-full max-h-full overflow-y-auto px-4 sm:px-6 md:px-8 py-4 space-y-6">
	<!-- Top Navigation -->
	<div>
		<button
			type="button"
			class="flex items-center gap-1.5 text-xs text-gray-500 hover:text-gray-900 dark:text-gray-400 dark:hover:text-gray-100 transition py-1"
			on:click={() => goto('/workspace/tools')}
		>
			<ChevronLeft className="size-3.5" strokeWidth="2" />
			<span>{$i18n.t('Tools')}</span>
		</button>
	</div>

	<!-- Header Banner -->
	<div class="flex flex-col sm:flex-row sm:items-center justify-between gap-4 pb-4 border-b border-gray-100 dark:border-gray-850">
		<div class="space-y-1">
			<div class="flex items-center gap-2">
				<h1 class="text-xl font-semibold text-gray-900 dark:text-gray-100">
					{editMode ? 'Edit MCP Tool Server' : 'Add MCP Tool Server'}
				</h1>
				<Badge type="info" content={type === 'mcp' ? 'SSE / Streamable' : 'OpenAPI'} />
			</div>
			<p class="text-xs text-gray-500 dark:text-gray-400">
				Hubungkan server MCP berbasis SSE / HTTP streamable atau OpenAPI untuk digunakan dalam percakapan AI secara otomatis.
			</p>
		</div>

		<!-- Action Buttons on Header -->
		<div class="flex flex-wrap items-center gap-2">
			{#if editMode}
				<button
					type="button"
					disabled={deleting}
					on:click={handleDelete}
					class="px-3.5 py-2 text-xs font-medium rounded-xl border border-red-200 dark:border-red-900/40 text-red-600 dark:text-red-400 hover:bg-red-50 dark:hover:bg-red-950/40 transition flex items-center gap-1.5"
				>
					{#if deleting}
						<Spinner className="size-3.5" />
					{:else}
						<GarbageBin className="size-3.5" />
					{/if}
					<span>Hapus</span>
				</button>
			{/if}

			<button
				type="button"
				disabled={verifying}
				on:click={handleTestConnection}
				class="px-3.5 py-2 text-xs font-medium rounded-xl border border-gray-200 dark:border-gray-800 hover:bg-gray-100 dark:hover:bg-gray-850 text-gray-700 dark:text-gray-300 transition flex items-center gap-1.5"
			>
				{#if verifying}
					<Spinner className="size-3.5" />
					<span>Memeriksa...</span>
				{:else}
					<svg xmlns="http://www.w3.org/2000/svg" viewBox="0 0 20 20" fill="currentColor" class="size-3.5">
						<path fill-rule="evenodd" d="M10 18a8 8 0 1 0 0-16 8 8 0 0 0 0 16Zm3.857-9.809a.75.75 0 0 0-1.214-.882l-3.483 4.79-1.88-1.88a.75.75 0 1 0-1.06 1.061l2.5 2.5a.75.75 0 0 0 1.137-.089l4-5.5Z" clip-rule="evenodd" />
					</svg>
					<span>Test & Check Tools</span>
				{/if}
			</button>

			<button
				type="button"
				disabled={saving}
				on:click={handleSave}
				class="px-4 py-2 text-xs font-medium rounded-xl bg-black hover:bg-gray-900 text-white dark:bg-white dark:text-black dark:hover:bg-gray-100 transition shadow-xs flex items-center gap-1.5"
			>
				{#if saving}
					<Spinner className="size-3.5" />
					<span>Menyimpan...</span>
				{:else}
					<span>{editMode ? 'Simpan Perubahan' : 'Simpan MCP Server'}</span>
				{/if}
			</button>
		</div>
	</div>

	<!-- Demo & Recommendation Box -->
	<div class="rounded-2xl border border-blue-100 dark:border-blue-900/30 bg-blue-50/40 dark:bg-blue-950/20 p-4 space-y-3">
		<div class="flex flex-col sm:flex-row sm:items-center justify-between gap-2">
			<div class="flex items-center gap-2">
				<span class="flex h-2 w-2 rounded-full bg-blue-500 animate-pulse"></span>
				<span class="text-xs font-semibold text-blue-900 dark:text-blue-300">Rekomendasi & Demo MCP:</span>
			</div>
			<div class="flex flex-wrap items-center gap-2">
				<button
					type="button"
					on:click={fillDemoUrl}
					class="text-xs px-2.5 py-1 rounded-lg bg-blue-100 dark:bg-blue-900/50 hover:bg-blue-200 dark:hover:bg-blue-800/60 text-blue-700 dark:text-blue-300 font-medium transition flex items-center gap-1"
				>
					<span>Gunakan Demo Bridge URL</span>
				</button>
				<a
					href={ARIA_URL}
					target="_blank"
					rel="noopener noreferrer"
					class="text-xs px-2.5 py-1 rounded-lg bg-purple-100 dark:bg-purple-900/50 hover:bg-purple-200 dark:hover:bg-purple-800/60 text-purple-700 dark:text-purple-300 font-medium transition flex items-center gap-1"
				>
					<span>Lihat Aria Page Agent</span>
					<svg xmlns="http://www.w3.org/2000/svg" viewBox="0 0 20 20" fill="currentColor" class="size-3">
						<path fill-rule="evenodd" d="M4.25 5.5a.75.75 0 0 0-.75.75v8.5c0 .414.336.75.75.75h8.5a.75.75 0 0 0 .75-.75v-4a.75.75 0 0 1 1.5 0v4A2.25 2.25 0 0 1 12.75 17h-8.5A2.25 2.25 0 0 1 2 14.75v-8.5A2.25 2.25 0 0 1 4.25 4h4a.75.75 0 0 1 0 1.5h-4Z" clip-rule="evenodd" />
						<path fill-rule="evenodd" d="M6.194 12.753a.75.75 0 0 0 1.06 1.06l7.25-7.25V9a.75.75 0 0 0 1.5 0V4.25a.75.75 0 0 0-.75-.75H10.5a.75.75 0 0 0 0 1.5h2.44l-6.746 6.753Z" clip-rule="evenodd" />
					</svg>
				</a>
			</div>
		</div>
		<p class="text-[11px] text-blue-800/80 dark:text-blue-300/80 leading-relaxed">
			Dukung SSE dan HTTP streamable transport seperti <code class="px-1 py-0.5 rounded bg-blue-100/70 dark:bg-blue-900/60 font-mono text-[10px]">mcp-bridge-go</code> atau <code class="px-1 py-0.5 rounded bg-blue-100/70 dark:bg-blue-900/60 font-mono text-[10px]">aria-page-agent</code>. AI dapat otomatis mengontrol peramban dan mengambil aksi langsung melalui bridge.
		</p>
	</div>

	<!-- Configuration Form Card -->
	<div class="rounded-2xl border border-gray-200/80 dark:border-gray-800/80 bg-white dark:bg-gray-900 p-4 sm:p-6 shadow-xs space-y-5">
		<h2 class="text-sm font-semibold text-gray-900 dark:text-gray-100">Konfigurasi Endpoint</h2>

		<!-- Protocol Selector -->
		<div class="space-y-1.5">
			<label for="server-type" class="block text-xs font-medium text-gray-500 dark:text-gray-400">Protokol Server</label>
			<div class="grid grid-cols-2 gap-2 max-w-md">
				<button
					type="button"
					on:click={() => (type = 'mcp')}
					class="py-2 px-3 rounded-xl text-xs font-semibold border transition flex items-center justify-center gap-1.5 {type === 'mcp'
						? 'bg-black text-white dark:bg-white dark:text-black border-transparent shadow-xs'
						: 'border-gray-200 dark:border-gray-800 hover:bg-gray-50 dark:hover:bg-gray-850 text-gray-700 dark:text-gray-300'}"
				>
					<svg xmlns="http://www.w3.org/2000/svg" viewBox="0 0 20 20" fill="currentColor" class="size-3.5">
						<path d="M14 6H6v8h8V6Z" />
						<path fill-rule="evenodd" d="M9.25 3V1.75a.75.75 0 0 1 1.5 0V3h1.5V1.75a.75.75 0 0 1 1.5 0V3h.5A2.75 2.75 0 0 1 17 5.75v.5h1.25a.75.75 0 0 1 0 1.5H17v1.5h1.25a.75.75 0 0 1 0 1.5H17v1.5h1.25a.75.75 0 0 1 0 1.5H17v.5A2.75 2.75 0 0 1 14.25 17h-.5v1.25a.75.75 0 0 1-1.5 0V17h-1.5v1.25a.75.75 0 0 1-1.5 0V17h-1.5v1.25a.75.75 0 0 1-1.5 0V17h-.5A2.75 2.75 0 0 1 3 14.25v-.5H1.75a.75.75 0 0 1 0-1.5H3v-1.5H1.75a.75.75 0 0 1 0-1.5H3v-1.5H1.75a.75.75 0 0 1 0-1.5H3v-.5A2.75 2.75 0 0 1 5.75 3h.5V1.75a.75.75 0 0 1 1.5 0V3h1.5ZM4.5 5.75c0-.69.56-1.25 1.25-1.25h8.5c.69 0 1.25.56 1.25 1.25v8.5c0 .69-.56 1.25-1.25 1.25h-8.5c-.69 0-1.25-.56-1.25-1.25v-8.5Z" clip-rule="evenodd" />
					</svg>
					<span>MCP (SSE / Streamable)</span>
				</button>
				<button
					type="button"
					on:click={() => (type = 'openapi')}
					class="py-2 px-3 rounded-xl text-xs font-semibold border transition flex items-center justify-center gap-1.5 {type === 'openapi'
						? 'bg-black text-white dark:bg-white dark:text-black border-transparent shadow-xs'
						: 'border-gray-200 dark:border-gray-800 hover:bg-gray-50 dark:hover:bg-gray-850 text-gray-700 dark:text-gray-300'}"
				>
					<svg xmlns="http://www.w3.org/2000/svg" viewBox="0 0 20 20" fill="currentColor" class="size-3.5">
						<path fill-rule="evenodd" d="M4.5 2A1.5 1.5 0 0 0 3 3.5v13A1.5 1.5 0 0 0 4.5 18h11a1.5 1.5 0 0 0 1.5-1.5V7.621a1.5 1.5 0 0 0-.44-1.06l-4.12-4.122A1.5 1.5 0 0 0 11.378 2H4.5Zm4.75 6.75a.75.75 0 0 1 .75.75v3.25a.75.75 0 0 1-1.5 0V9.5a.75.75 0 0 1 .75-.75Zm3 1.75a.75.75 0 0 0-1.5 0v1.5a.75.75 0 0 0 1.5 0v-1.5ZM7.5 11.25a.75.75 0 0 1 .75.75v.5a.75.75 0 0 1-1.5 0v-.5a.75.75 0 0 1 .75-.75Z" clip-rule="evenodd" />
					</svg>
					<span>OpenAPI Specification</span>
				</button>
			</div>
		</div>

		<!-- Endpoint URL Input with Inline Test Button -->
		<div class="space-y-1.5">
			<div class="flex items-center justify-between">
				<label for="server-url" class="block text-xs font-medium text-gray-500 dark:text-gray-400">
					Server URL <span class="text-red-500">*</span>
				</label>
				{#if verified}
					<span class="text-[11px] text-green-600 dark:text-green-400 font-medium flex items-center gap-1">
						<span class="size-1.5 rounded-full bg-green-500"></span>
						Terverifikasi {latencyMs ? `(${latencyMs} ms)` : ''}
					</span>
				{/if}
			</div>

			<div class="flex flex-col sm:flex-row gap-2">
				<div class="relative flex-1">
					<input
						id="server-url"
						type="text"
						bind:value={url}
						on:input={() => {
							verified = false;
							verifyError = '';
							autoSuggestName();
						}}
						placeholder="https://public-mcp-bridge.warunglakku.com/mcp?room=bc61a142"
						class="w-full text-sm rounded-xl border border-gray-200 dark:border-gray-800 bg-transparent px-3.5 py-2.5 outline-hidden focus:border-gray-400 dark:focus:border-gray-600 transition min-h-[42px]"
					/>
				</div>

				<button
					type="button"
					disabled={verifying || !url.trim()}
					on:click={handleTestConnection}
					class="px-4 py-2.5 text-xs font-medium rounded-xl border border-gray-200 dark:border-gray-800 hover:bg-gray-100 dark:hover:bg-gray-850 text-gray-700 dark:text-gray-300 transition shrink-0 flex items-center justify-center gap-1.5 disabled:opacity-50 min-h-[42px]"
				>
					{#if verifying}
						<Spinner className="size-3.5" />
						<span>Cek Tools...</span>
					{:else}
						<span>Uji Koneksi</span>
					{/if}
				</button>
			</div>

			{#if verifyError}
				<div class="rounded-xl bg-red-50 dark:bg-red-950/40 border border-red-200 dark:border-red-900/40 p-3 text-xs text-red-600 dark:text-red-400 leading-relaxed flex items-start gap-2">
					<svg xmlns="http://www.w3.org/2000/svg" viewBox="0 0 20 20" fill="currentColor" class="size-4 shrink-0 mt-0.5">
						<path fill-rule="evenodd" d="M18 10a8 8 0 1 1-16 0 8 8 0 0 1 16 0Zm-8-5a.75.75 0 0 1 .75.75v4.5a.75.75 0 0 1-1.5 0v-4.5A.75.75 0 0 1 10 5Zm0 10a1 1 0 1 0 0-2 1 1 0 0 0 0 2Z" clip-rule="evenodd" />
					</svg>
					<div class="flex-1 break-all">{verifyError}</div>
				</div>
			{/if}
		</div>

		<!-- Name, API Key, Enable Switch -->
		<div class="grid grid-cols-1 sm:grid-cols-2 gap-4">
			<div class="space-y-1.5">
				<label for="server-name" class="block text-xs font-medium text-gray-500 dark:text-gray-400">
					Nama Server
				</label>
				<input
					id="server-name"
					type="text"
					bind:value={name}
					placeholder="cth: Public MCP Bridge / Aria Browser"
					class="w-full text-sm rounded-xl border border-gray-200 dark:border-gray-800 bg-transparent px-3.5 py-2.5 outline-hidden focus:border-gray-400 dark:focus:border-gray-600 transition min-h-[42px]"
				/>
			</div>

			<div class="space-y-1.5">
				<label for="server-key" class="block text-xs font-medium text-gray-500 dark:text-gray-400">
					API Key / Auth Token <span class="text-gray-400 font-normal">(opsional)</span>
				</label>
				<input
					id="server-key"
					type="password"
					bind:value={key}
					placeholder="Bearer Token jika server memerlukan otentikasi"
					class="w-full text-sm rounded-xl border border-gray-200 dark:border-gray-800 bg-transparent px-3.5 py-2.5 outline-hidden focus:border-gray-400 dark:focus:border-gray-600 transition min-h-[42px]"
				/>
			</div>
		</div>

		<!-- Server Enable Switch -->
		<div class="flex items-center justify-between pt-2 border-t border-gray-100 dark:border-gray-850">
			<div>
				<div class="text-xs font-medium text-gray-800 dark:text-gray-200">Status Server Aktif</div>
				<div class="text-[11px] text-gray-500 dark:text-gray-400">
					Saat dinonaktifkan, model AI tidak akan dapat memanggil tools dari server ini.
				</div>
			</div>
			<Switch bind:state={enable} />
		</div>
	</div>

	<!-- Discovered Tools with Enable / Disable Toggles -->
	<div class="rounded-2xl border border-gray-200/80 dark:border-gray-800/80 bg-white dark:bg-gray-900 p-4 sm:p-6 shadow-xs space-y-4">
		<div class="flex flex-col sm:flex-row sm:items-center justify-between gap-3 pb-3 border-b border-gray-100 dark:border-gray-850">
			<div class="space-y-0.5">
				<div class="flex items-center gap-2">
					<h2 class="text-sm font-semibold text-gray-900 dark:text-gray-100">Daftar Tool Tersedia</h2>
					{#if discoveredTools.length > 0}
						<span class="px-2 py-0.5 rounded-full text-[11px] font-semibold bg-gray-100 dark:bg-gray-800 text-gray-700 dark:text-gray-300">
							{activeToolsCount} / {discoveredTools.length} Aktif
						</span>
					{/if}
				</div>
				<p class="text-xs text-gray-500 dark:text-gray-400">
					Aktifkan atau nonaktifkan tool individual sesuai kebutuhan model percakapan Anda.
				</p>
			</div>

			{#if discoveredTools.length > 0}
				<div class="flex items-center gap-2">
					<button
						type="button"
						on:click={enableAllTools}
						class="px-2.5 py-1 text-xs font-medium rounded-lg border border-gray-200 dark:border-gray-800 hover:bg-gray-100 dark:hover:bg-gray-850 text-gray-600 dark:text-gray-300 transition"
					>
						Aktifkan Semua
					</button>
					<button
						type="button"
						on:click={disableAllTools}
						class="px-2.5 py-1 text-xs font-medium rounded-lg border border-gray-200 dark:border-gray-800 hover:bg-gray-100 dark:hover:bg-gray-850 text-gray-600 dark:text-gray-300 transition"
					>
						Nonaktifkan Semua
					</button>
				</div>
			{/if}
		</div>

		<!-- Search Bar for Tools if > 3 tools -->
		{#if discoveredTools.length > 3}
			<div class="relative">
				<Search className="absolute left-3 top-3 size-3.5 text-gray-400" />
				<input
					type="text"
					bind:value={toolSearchQuery}
					placeholder="Cari tool berdasarkan nama atau deskripsi..."
					class="w-full text-xs rounded-xl border border-gray-200 dark:border-gray-800 bg-transparent pl-9 pr-3.5 py-2 outline-hidden focus:border-gray-400 dark:focus:border-gray-600 transition"
				/>
			</div>
		{/if}

		<!-- Tools List Cards -->
		{#if discoveredTools.length > 0}
			<div class="space-y-3">
				{#each filteredTools as tool, idx}
					{@const enabled = isToolEnabled(tool.name)}
					<div class="rounded-xl border transition p-3.5 {enabled
						? 'border-gray-200 dark:border-gray-800 bg-gray-50/50 dark:bg-gray-850/50'
						: 'border-gray-200/50 dark:border-gray-800/40 bg-gray-50/20 dark:bg-gray-850/20 opacity-70'}">
						<div class="flex items-start justify-between gap-3">
							<div class="space-y-1.5 flex-1 min-w-0">
								<div class="flex flex-wrap items-center gap-2">
									<code class="px-2 py-0.5 rounded-md font-mono text-xs font-semibold bg-gray-200/70 dark:bg-gray-800 text-gray-900 dark:text-gray-100">
										{tool.name}
									</code>

									{#if enabled}
										<span class="text-[10px] font-medium px-1.5 py-0.5 rounded bg-green-100 dark:bg-green-950/60 text-green-700 dark:text-green-400">
											Aktif
										</span>
									{:else}
										<span class="text-[10px] font-medium px-1.5 py-0.5 rounded bg-gray-100 dark:bg-gray-800 text-gray-500 dark:text-gray-400">
											Nonaktif
										</span>
									{/if}
								</div>

								<p class="text-xs text-gray-600 dark:text-gray-300 leading-relaxed">
									{tool.description || 'Tidak ada deskripsi tool.'}
								</p>
							</div>

							<!-- Individual Tool Toggle Switch -->
							<div class="shrink-0 pt-0.5">
								<Switch
									state={enabled}
									on:change={(e) => toggleToolStatus(tool.name, e.detail)}
								/>
							</div>
						</div>

						<!-- Expandable Parameters Schema -->
						{#if tool.inputSchema && Object.keys(tool.inputSchema.properties || {}).length > 0}
							<div class="mt-3 pt-2.5 border-t border-gray-200/50 dark:border-gray-800/50">
								<button
									type="button"
									on:click={() => (expandedToolIdx = expandedToolIdx === idx ? null : idx)}
									class="text-[11px] text-gray-500 hover:text-gray-900 dark:hover:text-gray-200 font-medium flex items-center gap-1 transition"
								>
									{#if expandedToolIdx === idx}
										<ChevronUp className="size-3" />
										<span>Sembunyikan Parameter ({Object.keys(tool.inputSchema.properties).length})</span>
									{:else}
										<ChevronDown className="size-3" />
										<span>Lihat Parameter Schema ({Object.keys(tool.inputSchema.properties).length})</span>
									{/if}
								</button>

								{#if expandedToolIdx === idx}
									<div class="mt-2 rounded-lg bg-gray-900 text-gray-100 p-3 text-[11px] font-mono overflow-x-auto">
										<pre>{JSON.stringify(tool.inputSchema, null, 2)}</pre>
									</div>
								{/if}
							</div>
						{/if}
					</div>
				{/each}
			</div>
		{:else if verified}
			<div class="rounded-xl border border-amber-200 dark:border-amber-900/40 bg-amber-50/50 dark:bg-amber-950/20 p-4 text-xs text-amber-800 dark:text-amber-300 space-y-1">
				<div class="font-semibold flex items-center gap-1.5">
					<svg xmlns="http://www.w3.org/2000/svg" viewBox="0 0 20 20" fill="currentColor" class="size-4">
						<path fill-rule="evenodd" d="M18 10a8 8 0 1 1-16 0 8 8 0 0 1 16 0Zm-7-4a1 1 0 1 1-2 0 1 1 0 0 1 2 0ZM9 9a.75.75 0 0 0 0 1.5h.253a.25.25 0 0 1 .244.304l-.459 2.066A1.75 1.75 0 0 0 10.747 15H11a.75.75 0 0 0 0-1.5h-.253a.25.25 0 0 1-.244-.304l.459-2.066A1.75 1.75 0 0 0 9.253 9H9Z" clip-rule="evenodd" />
					</svg>
					<span>Server terhubung, namun belum ada tool yang dipublikasikan (0 tools)</span>
				</div>
				<p class="text-[11px] leading-relaxed text-amber-700 dark:text-amber-400">
					Untuk server jenis bridge seperti warunglakku atau Aria Agent, pastikan browser extension atau client worker sudah terhubung ke room yang sama agar tool muncul. Anda tetap dapat menyimpan server ini.
				</p>
			</div>
		{:else}
			<div class="py-8 text-center space-y-2 border border-dashed border-gray-200 dark:border-gray-800 rounded-xl">
				<div class="mx-auto size-9 rounded-full bg-gray-100 dark:bg-gray-800 flex items-center justify-center text-gray-400">
					<WrenchAlt className="size-4" />
				</div>
				<div class="text-xs font-medium text-gray-700 dark:text-gray-300">Belum ada tool yang diverifikasi</div>
				<p class="text-[11px] text-gray-500 dark:text-gray-400 max-w-sm mx-auto">
					Klik tombol <strong class="text-gray-700 dark:text-gray-200">"Test & Check Tools"</strong> untuk memeriksa koneksi dan menampilkan daftar tool yang tersedia secara otomatis.
				</p>
			</div>
		{/if}
	</div>

	<!-- Bottom Action Bar -->
	<div class="flex flex-col sm:flex-row items-center justify-between gap-3 pt-2 pb-12">
		<button
			type="button"
			on:click={() => goto('/workspace/tools')}
			class="w-full sm:w-auto px-4 py-2.5 text-xs font-medium rounded-xl border border-gray-200 dark:border-gray-800 hover:bg-gray-100 dark:hover:bg-gray-850 text-gray-700 dark:text-gray-300 transition"
		>
			Kembali ke Tools
		</button>

		<div class="flex flex-col sm:flex-row items-center gap-2 w-full sm:w-auto">
			{#if editMode}
				<button
					type="button"
					disabled={deleting}
					on:click={handleDelete}
					class="w-full sm:w-auto px-4 py-2.5 text-xs font-medium rounded-xl border border-red-200 dark:border-red-900/40 text-red-600 dark:text-red-400 hover:bg-red-50 dark:hover:bg-red-950/40 transition flex items-center justify-center gap-1.5"
				>
					{#if deleting}
						<Spinner className="size-3.5" />
					{:else}
						<GarbageBin className="size-3.5" />
					{/if}
					<span>Hapus Server</span>
				</button>
			{/if}

			<button
				type="button"
				disabled={saving}
				on:click={handleSave}
				class="w-full sm:w-auto px-5 py-2.5 text-xs font-medium rounded-xl bg-black hover:bg-gray-900 text-white dark:bg-white dark:text-black dark:hover:bg-gray-100 transition shadow-xs flex items-center justify-center gap-1.5"
			>
				{#if saving}
					<Spinner className="size-3.5" />
					<span>Menyimpan...</span>
				{:else}
					<span>{editMode ? 'Simpan Perubahan' : 'Simpan MCP Server'}</span>
				{/if}
			</button>
		</div>
	</div>
</div>
