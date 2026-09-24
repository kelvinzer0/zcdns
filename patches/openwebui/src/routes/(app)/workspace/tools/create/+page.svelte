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
	};

	const handleTestConnection = async () => {
		const targetUrl = url.trim();
		if (!targetUrl) {
			toast.error('Silakan masukkan Server URL terlebih dahulu');
			return false;
		}

		verifying = true;
		verifyError = '';
		discoveredTools = [];
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
			latencyMs = null;
			verified = false;
			const msg = err?.message || err?.detail || String(err);
			verifyError = msg;
			toast.error(`Koneksi gagal: ${msg}`);
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
		if (!name.trim()) {
			autoSuggestName();
			if (!name.trim()) {
				name = 'MCP Server';
			}
		}

		saving = true;
		try {
			// Fetch all current connections
			const res = await getToolServerConnections(localStorage.token).catch(() => null);
			let connections: any[] = res?.TOOL_SERVER_CONNECTIONS || [];

			const targetId = serverId || `mcp_${uuidv4().slice(0, 8)}`;

			const newServer = {
				id: targetId,
				name: name.trim(),
				type,
				url: targetUrl,
				auth_type: authType,
				key: key.trim(),
				config: {
					enable
				},
				info: {
					id: targetId,
					name: name.trim(),
					description: `MCP Server (${type}) via ${targetUrl}`
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
		if (!confirm('Apakah Anda yakin ingin menghapus server ini?')) return;

		deleting = true;
		try {
			const res = await getToolServerConnections(localStorage.token).catch(() => null);
			let connections: any[] = res?.TOOL_SERVER_CONNECTIONS || [];
			connections = connections.filter((s) => s.id !== serverId && s.info?.id !== serverId);

			await setToolServerConnections(localStorage.token, {
				TOOL_SERVER_CONNECTIONS: connections
			});

			toast.success('Server berhasil dihapus');
			goto('/workspace/tools');
		} catch (err: any) {
			toast.error(`Gagal menghapus server: ${err.message || err}`);
		} finally {
			deleting = false;
		}
	};

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

				// Otomatis verifikasi untuk melihat tool list saat edit / clone
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

<div class="max-w-4xl mx-auto py-6 px-4 space-y-6">
	<!-- Top Navigation / Breadcrumbs -->
	<div class="flex items-center justify-between pb-4 border-b border-gray-100 dark:border-gray-800">
		<div class="flex items-center gap-3">
			<a
				href="/workspace/tools"
				class="p-2 rounded-xl text-gray-500 hover:text-gray-900 dark:hover:text-gray-100 hover:bg-gray-100 dark:hover:bg-gray-800 transition"
				aria-label="Kembali"
			>
				<svg xmlns="http://www.w3.org/2000/svg" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round" class="size-5">
					<path d="m15 18-6-6 6-6" />
				</svg>
			</a>
			<div>
				<div class="flex items-center gap-2">
					<h1 class="text-xl font-bold text-gray-900 dark:text-gray-100">
						{editMode ? 'Edit Tool Server' : 'Add MCP Server'}
					</h1>
					<span class="text-[11px] font-semibold px-2 py-0.5 rounded-full bg-blue-100 text-blue-700 dark:bg-blue-900/50 dark:text-blue-300">
						SSE / Streamable HTTP
					</span>
				</div>
				<p class="text-xs text-gray-500 dark:text-gray-400 mt-0.5">
					Sambungkan alat bantu model AI melalui protokol MCP secara real-time dengan streaming data.
				</p>
			</div>
		</div>

		<!-- Quick demo badge button -->
		<button
			type="button"
			on:click={fillDemoUrl}
			class="hidden sm:inline-flex items-center gap-1.5 px-3 py-1.5 rounded-xl text-xs font-medium bg-blue-50 dark:bg-blue-950/60 text-blue-600 dark:text-blue-400 border border-blue-200/50 dark:border-blue-900/50 hover:bg-blue-100 dark:hover:bg-blue-900/60 transition"
		>
			<svg xmlns="http://www.w3.org/2000/svg" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round" class="size-3.5">
				<path d="m12 3-1.912 5.813a2 2 0 0 1-1.275 1.275L3 12l5.813 1.912a2 2 0 0 1 1.275 1.275L12 21l1.912-5.813a2 2 0 0 1 1.275-1.275L21 12l-5.813-1.912a2 2 0 0 1-1.275-1.275L12 3Z"/>
			</svg>
			<span>Gunakan Demo URL</span>
		</button>
	</div>

	<!-- Main Form Card -->
	<div class="bg-white dark:bg-gray-900 rounded-2xl border border-gray-200/80 dark:border-gray-800 p-6 space-y-6 shadow-sm">
		<!-- Protocol Selection -->
		<div class="space-y-1.5">
			<label class="text-xs font-semibold text-gray-700 dark:text-gray-300">
				Protokol Server
			</label>
			<div class="flex gap-2">
				<button
					type="button"
					on:click={() => { type = 'mcp'; verified = false; }}
					class="flex-1 py-2 px-3 rounded-xl text-xs font-semibold border transition flex items-center justify-center gap-2 {type === 'mcp'
						? 'bg-blue-50 dark:bg-blue-950/60 text-blue-600 dark:text-blue-400 border-blue-500'
						: 'border-gray-200 dark:border-gray-800 text-gray-600 dark:text-gray-400 hover:bg-gray-50 dark:hover:bg-gray-850'}"
				>
					<svg xmlns="http://www.w3.org/2000/svg" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round" class="size-4">
						<circle cx="12" cy="12" r="10" />
						<circle cx="12" cy="12" r="4" />
						<line x1="21.17" y1="8" x2="12" y2="8" />
						<line x1="3.95" y1="6.06" x2="8.54" y2="14" />
						<line x1="10.88" y1="21.94" x2="15.46" y2="14" />
					</svg>
					<span>MCP (Model Context Protocol — SSE / HTTP Streamable)</span>
				</button>

				<button
					type="button"
					on:click={() => { type = 'openapi'; verified = false; }}
					class="py-2 px-4 rounded-xl text-xs font-semibold border transition flex items-center justify-center gap-2 {type === 'openapi'
						? 'bg-blue-50 dark:bg-blue-950/60 text-blue-600 dark:text-blue-400 border-blue-500'
						: 'border-gray-200 dark:border-gray-800 text-gray-600 dark:text-gray-400 hover:bg-gray-50 dark:hover:bg-gray-850'}"
				>
					<span>OpenAPI</span>
				</button>
			</div>
		</div>

		<!-- Server URL Input -->
		<div class="space-y-1.5">
			<div class="flex items-center justify-between">
				<label for="server-url" class="text-xs font-semibold text-gray-700 dark:text-gray-300">
					Endpoint URL <span class="text-red-500">*</span>
				</label>
				<span class="text-[11px] text-gray-400">Mendukung SSE stream atau JSON-RPC HTTP</span>
			</div>
			<div class="flex gap-2">
				<input
					id="server-url"
					type="text"
					bind:value={url}
					on:input={() => { verified = false; }}
					placeholder="https://public-mcp-bridge.warunglakku.com/mcp?room=bc61a142"
					class="w-full text-xs font-mono px-3.5 py-2.5 rounded-xl border border-gray-200 dark:border-gray-800 bg-gray-50/50 dark:bg-gray-850/50 focus:outline-none focus:border-blue-500 text-gray-900 dark:text-gray-100"
				/>
				<button
					type="button"
					on:click={handleTestConnection}
					disabled={verifying || !url.trim()}
					class="inline-flex items-center gap-2 px-4 py-2.5 rounded-xl text-xs font-semibold bg-gray-900 hover:bg-black dark:bg-gray-100 dark:hover:bg-white text-white dark:text-gray-900 transition shrink-0 disabled:opacity-50 cursor-pointer"
				>
					{#if verifying}
						<svg class="animate-spin size-3.5" viewBox="0 0 24 24" fill="none">
							<circle class="opacity-25" cx="12" cy="12" r="10" stroke="currentColor" stroke-width="4"></circle>
							<path class="opacity-75" fill="currentColor" d="M4 12a8 8 0 018-8V0C5.373 0 0 5.373 0 12h4z"></path>
						</svg>
						<span>Memeriksa...</span>
					{:else}
						<svg xmlns="http://www.w3.org/2000/svg" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round" class="size-3.5">
							<polygon points="13 2 3 14 12 14 11 22 21 10 12 10 13 2"/>
						</svg>
						<span>Test & Periksa Tools</span>
					{/if}
				</button>
			</div>
		</div>

		<!-- Server Name & Options -->
		<div class="grid grid-cols-1 sm:grid-cols-2 gap-4">
			<div class="space-y-1.5">
				<label for="server-name" class="text-xs font-semibold text-gray-700 dark:text-gray-300">
					Nama Server
				</label>
				<input
					id="server-name"
					type="text"
					bind:value={name}
					placeholder="Contoh: Aria Browser Agent atau MCP Bridge"
					class="w-full text-xs px-3.5 py-2.5 rounded-xl border border-gray-200 dark:border-gray-800 bg-gray-50/50 dark:bg-gray-850/50 focus:outline-none focus:border-blue-500 text-gray-900 dark:text-gray-100"
				/>
			</div>

			<div class="space-y-1.5">
				<label for="server-key" class="text-xs font-semibold text-gray-700 dark:text-gray-300">
					API Key / Bearer Token <span class="text-gray-400 font-normal">(Opsional)</span>
				</label>
				<input
					id="server-key"
					type="password"
					bind:value={key}
					placeholder="Authorization token jika diperlukan"
					class="w-full text-xs px-3.5 py-2.5 rounded-xl border border-gray-200 dark:border-gray-800 bg-gray-50/50 dark:bg-gray-850/50 focus:outline-none focus:border-blue-500 text-gray-900 dark:text-gray-100"
				/>
			</div>
		</div>

		<!-- Status Toggle -->
		<div class="flex items-center justify-between pt-2">
			<div>
				<div class="text-xs font-semibold text-gray-800 dark:text-gray-200">Aktifkan Koneksi</div>
				<div class="text-[11px] text-gray-400">Aktifkan server ini agar dapat digunakan oleh model AI saat chatting</div>
			</div>
			<input
				type="checkbox"
				bind:checked={enable}
				class="size-4 rounded text-blue-600 focus:ring-blue-500"
			/>
		</div>
	</div>

	<!-- Verification & Discovered Tools Result Card -->
	<div class="bg-white dark:bg-gray-900 rounded-2xl border border-gray-200/80 dark:border-gray-800 p-6 space-y-4 shadow-sm">
		<div class="flex items-center justify-between">
			<div class="flex items-center gap-2">
				<h2 class="text-sm font-bold text-gray-900 dark:text-gray-100">
					Daftar Tool yang Tersedia
				</h2>
				{#if verified}
					<span class="text-[11px] font-semibold px-2.5 py-0.5 rounded-full bg-emerald-100 text-emerald-700 dark:bg-emerald-950/60 dark:text-emerald-300 flex items-center gap-1">
						<span class="size-1.5 rounded-full bg-emerald-500"></span>
						Terverifikasi {latencyMs ? `(${latencyMs}ms)` : ''}
					</span>
				{:else if verifyError}
					<span class="text-[11px] font-semibold px-2.5 py-0.5 rounded-full bg-red-100 text-red-700 dark:bg-red-950/60 dark:text-red-300">
						Gagal Terhubung
					</span>
				{/if}
			</div>

			{#if verified}
				<span class="text-xs text-gray-500 dark:text-gray-400">
					{discoveredTools.length} tool terdeteksi
				</span>
			{/if}
		</div>

		<!-- Error display -->
		{#if verifyError}
			<div class="p-4 rounded-xl bg-red-50 dark:bg-red-950/40 border border-red-200 dark:border-red-900 text-xs text-red-700 dark:text-red-300 flex items-start gap-2.5">
				<svg xmlns="http://www.w3.org/2000/svg" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round" class="size-4 shrink-0 mt-0.5 text-red-500">
					<circle cx="12" cy="12" r="10" />
					<line x1="12" y1="8" x2="12" y2="12" />
					<line x1="12" y1="16" x2="12.01" y2="16" />
				</svg>
				<div class="space-y-1">
					<div class="font-semibold">Tidak dapat terhubung ke server MCP:</div>
					<div class="font-mono text-[11px] break-all">{verifyError}</div>
					<div class="text-[11px] text-gray-500 dark:text-gray-400 mt-1">
						Pastikan URL benar, extension atau bridge aktif, dan jaringan memperbolehkan CORS.
					</div>
				</div>
			</div>
		{/if}

		<!-- Discovered tools list -->
		{#if verified}
			{#if discoveredTools.length === 0}
				<div class="p-4 rounded-xl bg-blue-50/60 dark:bg-blue-950/30 border border-blue-200/50 dark:border-blue-900/40 text-xs text-blue-700 dark:text-blue-300 flex items-start gap-2.5">
					<svg xmlns="http://www.w3.org/2000/svg" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round" class="size-4 shrink-0 mt-0.5 text-blue-500">
						<circle cx="12" cy="12" r="10" />
						<line x1="12" y1="16" x2="12" y2="12" />
						<line x1="12" y1="8" x2="12.01" y2="8" />
					</svg>
					<div class="space-y-1">
						<div class="font-semibold">Koneksi MCP Berhasil (0 Tool Terpublikasi Saat Ini)</div>
						<div class="text-[11px] leading-relaxed text-gray-600 dark:text-gray-400">
							Server MCP merespons dengan format JSON-RPC 2.0 yang valid, namun saat ini belum ada fungsi tool yang didaftarkan di room/sesi ini. Anda tetap dapat menyimpan server ini; tool akan terbaca otomatis begitu terdaftar di sisi agen/ekstensi.
						</div>
					</div>
				</div>
			{:else}
				<div class="space-y-2.5">
					{#each discoveredTools as tool, idx}
						<div class="p-3 rounded-xl border border-gray-200/70 dark:border-gray-800 bg-gray-50/40 dark:bg-gray-850/40 space-y-2">
							<div class="flex items-center justify-between">
								<div class="flex items-center gap-2">
									<span class="font-mono text-xs font-bold text-blue-600 dark:text-blue-400 bg-blue-50 dark:bg-blue-950/60 px-2 py-0.5 rounded-lg border border-blue-200/50 dark:border-blue-900/50">
										{tool.name}
									</span>
								</div>
								{#if tool.inputSchema?.properties}
									<button
										type="button"
										on:click={() => { expandedToolIdx = expandedToolIdx === idx ? null : idx; }}
										class="text-[11px] text-gray-500 hover:text-gray-900 dark:hover:text-gray-200 underline cursor-pointer"
									>
										{expandedToolIdx === idx ? 'Tutup Parameter' : 'Lihat Parameter'}
									</button>
								{/if}
							</div>
							<p class="text-xs text-gray-600 dark:text-gray-300">
								{tool.description || 'Tidak ada deskripsi tool'}
							</p>

							{#if expandedToolIdx === idx && tool.inputSchema}
								<div class="p-2.5 rounded-lg bg-white dark:bg-gray-900 border border-gray-100 dark:border-gray-800 font-mono text-[11px] overflow-x-auto">
									<pre>{JSON.stringify(tool.inputSchema, null, 2)}</pre>
								</div>
							{/if}
						</div>
					{/each}
				</div>
			{/if}
		{:else if !verifyError}
			<div class="py-8 text-center text-xs text-gray-400 space-y-2">
				<svg xmlns="http://www.w3.org/2000/svg" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="1.5" stroke-linecap="round" stroke-linejoin="round" class="size-8 mx-auto text-gray-300 dark:text-gray-600">
					<circle cx="12" cy="12" r="10" />
					<path d="M12 16v-4" />
					<path d="M12 8h.01" />
				</svg>
				<div>Klik tombol <strong>"Test & Periksa Tools"</strong> di atas untuk memverifikasi endpoint dan melihat daftar tool yang tersedia sebelum menyimpan.</div>
			</div>
		{/if}
	</div>

	<!-- Action Footer Buttons -->
	<div class="flex items-center justify-between pt-2">
		<div>
			{#if editMode}
				<button
					type="button"
					on:click={handleDelete}
					disabled={deleting}
					class="px-4 py-2.5 rounded-xl text-xs font-semibold text-red-600 hover:bg-red-50 dark:hover:bg-red-950/40 border border-red-200 dark:border-red-900 transition cursor-pointer"
				>
					Hapus Server
				</button>
			{/if}
		</div>

		<div class="flex items-center gap-3">
			<a
				href="/workspace/tools"
				class="px-4 py-2.5 rounded-xl text-xs font-semibold text-gray-600 dark:text-gray-400 hover:bg-gray-100 dark:hover:bg-gray-800 transition"
			>
				Batal
			</a>

			<button
				type="button"
				on:click={handleSave}
				disabled={saving || !url.trim()}
				class="inline-flex items-center gap-2 px-6 py-2.5 rounded-xl text-xs font-semibold bg-blue-600 hover:bg-blue-700 text-white transition shadow-sm disabled:opacity-50 cursor-pointer"
			>
				{#if saving}
					<svg class="animate-spin size-3.5" viewBox="0 0 24 24" fill="none">
						<circle class="opacity-25" cx="12" cy="12" r="10" stroke="currentColor" stroke-width="4"></circle>
						<path class="opacity-75" fill="currentColor" d="M4 12a8 8 0 018-8V0C5.373 0 0 5.373 0 12h4z"></path>
					</svg>
					<span>Menyimpan...</span>
				{:else}
					<span>Simpan Server MCP</span>
				{/if}
			</button>
		</div>
	</div>
</div>
