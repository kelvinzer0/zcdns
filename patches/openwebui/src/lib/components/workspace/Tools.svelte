<script lang="ts">
	import { resolveLocalizedResource } from '$lib/utils/localizedContent';
	import dayjs from 'dayjs';
	import relativeTime from 'dayjs/plugin/relativeTime';
	import { toast } from 'svelte-sonner';
	import fileSaver from 'file-saver';
	const { saveAs } = fileSaver;

	dayjs.extend(relativeTime);

	import { onMount, getContext, tick, onDestroy } from 'svelte';
	const i18n = getContext('i18n');

	import {
		WEBUI_NAME,
		config,
		tools as _tools,
		user,
		workspaceActions,
		workspaceCounts
	} from '$lib/stores';

	import { goto } from '$app/navigation';
	import {
		createNewTool,
		loadToolByUrl,
		deleteToolById,
		exportTools,
		getToolById,
		getToolList,
		getTools
	} from '$lib/apis/tools';
	import { capitalizeFirstLetter } from '$lib/utils';

	import Tooltip from '../common/Tooltip.svelte';
	import ConfirmDialog from '../common/ConfirmDialog.svelte';
	import ToolMenu from './Tools/ToolMenu.svelte';
	import EllipsisHorizontal from '../icons/EllipsisHorizontal.svelte';
	import ValvesModal from './common/ValvesModal.svelte';
	import ManifestModal from './common/ManifestModal.svelte';
	import Heart from '../icons/Heart.svelte';
	import DeleteConfirmDialog from '$lib/components/common/ConfirmDialog.svelte';
	import GarbageBin from '../icons/GarbageBin.svelte';
	import Search from '../icons/Search.svelte';
	import Spinner from '../common/Spinner.svelte';
	import XMark from '../icons/XMark.svelte';
	import ImportModal from '../ImportModal.svelte';
	import ViewSelector from './common/ViewSelector.svelte';
	import CommunityDiscover from './common/CommunityDiscover.svelte';
	import Badge from '$lib/components/common/Badge.svelte';
	import ChevronDown from '../icons/ChevronDown.svelte';
	import ChevronUp from '../icons/ChevronUp.svelte';
	import Plus from '../icons/Plus.svelte';
	import WrenchAlt from '../icons/WrenchAlt.svelte';
	import Cog6 from '../icons/Cog6.svelte';
	import Switch from '$lib/components/common/Switch.svelte';
	import { getToolServerConnections, setToolServerConnections } from '$lib/apis/configs';

	let shiftKey = false;
	let loaded = false;

	let toolServers: any[] = [];

	const fetchToolServers = async () => {
		const res = await getToolServerConnections(localStorage.token).catch(() => null);
		if (res && res.TOOL_SERVER_CONNECTIONS) {
			toolServers = res.TOOL_SERVER_CONNECTIONS;
		} else {
			toolServers = [];
		}
	};

	const saveToolServers = async (newServers: any[]) => {
		const res = await setToolServerConnections(localStorage.token, {
			TOOL_SERVER_CONNECTIONS: newServers
		}).catch((err) => {
			toast.error($i18n.t('Failed to save connections'));
			return null;
		});

		if (res) {
			toast.success($i18n.t('Connection saved successfully'));
			await fetchToolServers();
			await init();
		}
	};

	const deleteToolServer = async (serverId: string) => {
		let updated = toolServers.filter((s) => (s.id || s.info?.id) !== serverId);
		await saveToolServers(updated);
	};

	const toggleToolServer = async (server: any, enabled: boolean) => {
		const sId = server.id || server.info?.id;
		let updated = toolServers.map((s) => {
			if ((s.id || s.info?.id) === sId) {
				return {
					...s,
					config: {
						...(s.config || {}),
						enable: enabled
					}
				};
			}
			return s;
		});
		await saveToolServers(updated);
	};

	let toolsImportInputElement: HTMLInputElement;
	let importFiles;

	let showConfirm = false;
	let query = '';
	let searchDebounceTimer: ReturnType<typeof setTimeout>;

	let showManifestModal = false;
	let showValvesModal = false;
	let selectedTool = null;

	let showDeleteConfirm = false;

	let tools = [];
	let filteredItems = [];

	let tagsContainerElement: HTMLDivElement;
	let viewOption = '';
	let sortKey = 'updated_at';
	let sortDirection = 'desc';
	let openToolMenuId: string | null = null;

	let showImportModal = false;

	$: if (loaded) {
		workspaceActions.set([
			{
				id: 'tools-add-mcp',
				label: 'Add MCP Server',
				href: '/workspace/tools/create'
			}
		]);
	}

	const handleSearchInput = () => {
		clearTimeout(searchDebounceTimer);
		searchDebounceTimer = setTimeout(() => {
			setFilteredItems();
		}, 300);
	};

	$: if (
		tools &&
		viewOption !== undefined &&
		sortKey !== undefined &&
		sortDirection !== undefined
	) {
		setFilteredItems();
	}

	const setFilteredItems = () => {
		const filtered = tools.filter((t) => {
			if (query === '' && viewOption === '') return true;
			const lowerQuery = query.toLowerCase();
			return (
				((t.name || '').toLowerCase().includes(lowerQuery) ||
					resolveLocalizedResource(t, $i18n.language).toLowerCase().includes(lowerQuery) ||
					(t.id || '').toLowerCase().includes(lowerQuery) ||
					(t.user?.name || '').toLowerCase().includes(lowerQuery) || // Search by user name
					(t.user?.email || '').toLowerCase().includes(lowerQuery)) && // Search by user email
				(viewOption === '' ||
					(viewOption === 'created' && t.user_id === $user?.id) ||
					(viewOption === 'shared' && t.user_id !== $user?.id))
			);
		});

		filteredItems = [...filtered].sort((a, b) => {
			const direction = sortDirection === 'asc' ? 1 : -1;

			if (sortKey === 'name') {
				return direction * (a.name ?? '').localeCompare(b.name ?? '');
			}

			return direction * ((a.updated_at ?? 0) - (b.updated_at ?? 0));
		});

		workspaceCounts.update((counts) => ({ ...counts, tools: filteredItems.length }));
	};

	const setSortKey = (key: string) => {
		if (sortKey === key) {
			sortDirection = sortDirection === 'asc' ? 'desc' : 'asc';
		} else {
			sortKey = key;
			sortDirection = key === 'updated_at' ? 'desc' : 'asc';
		}
	};

	const openTool = (tool) => {
		const cleanId = tool.id.replace('server:mcp:', '');
		goto(`/workspace/tools/create?id=${encodeURIComponent(cleanId)}`);
	};

	const shouldIgnoreRowClick = (target: EventTarget | null) => {
		return target instanceof Element && !!target.closest('button, a, input, [role="menu"]');
	};

	const shareHandler = async (tool) => {
		toast.info('MCP Tool: ' + (tool.name || tool.id));
	};

	const cloneHandler = async (tool) => {
		const cleanId = tool.id.replace('server:mcp:', '');
		const server = toolServers.find(
			(s) => s.id === cleanId || s.id === tool.id || s.info?.id === cleanId || s.info?.id === tool.id
		);
		if (server) {
			goto(`/workspace/tools/create?id=${encodeURIComponent(server.id || server.info?.id)}&clone=true`);
		}
	};

	const exportHandler = async (tool) => {
		const _tool = await getToolById(localStorage.token, tool.id).catch((error) => {
			toast.error(`${error}`);
			return null;
		});

		if (_tool) {
			let blob = new Blob([JSON.stringify([_tool])], {
				type: 'application/json'
			});
			saveAs(blob, `tool-${_tool.id}-export-${Date.now()}.json`);
		}
	};

	const deleteHandler = async (tool) => {
		const res = await deleteToolById(localStorage.token, tool.id).catch((error) => {
			toast.error(`${error}`);
			return null;
		});

		if (res) {
			toast.success($i18n.t('Tool deleted successfully'));
			await init();
		}
	};

	const init = async () => {
		tools = await getToolList(localStorage.token);
		_tools.set(await getTools(localStorage.token));
	};

	onMount(async () => {
		viewOption = localStorage?.workspaceViewOption || '';
		await fetchToolServers();
		await init();
		loaded = true;

		if (typeof window !== 'undefined' && window.location.search.includes('action=add-mcp')) {
			goto('/workspace/tools/create');
		}

		const onKeyDown = (event) => {
			if (event.key === 'Shift') {
				shiftKey = true;
			}
		};

		const onKeyUp = (event) => {
			if (event.key === 'Shift') {
				shiftKey = false;
			}
		};

		const onBlur = () => {
			shiftKey = false;
		};

		window.addEventListener('keydown', onKeyDown);
		window.addEventListener('keyup', onKeyUp);
		window.addEventListener('blur', onBlur);

		return () => {
			clearTimeout(searchDebounceTimer);
			window.removeEventListener('keydown', onKeyDown);
			window.removeEventListener('keyup', onKeyUp);
			window.removeEventListener('blur', onBlur);
		};
	});

	onDestroy(() => {
		clearTimeout(searchDebounceTimer);
	});
</script>

<svelte:head>
	<!-- LICENSE covers this Open WebUI browser-title identifier.
	Do not alter, remove, obscure, or replace it except as LICENSE permits:
	https://docs.openwebui.com/license. -->
	<title>
		{$i18n.t('Tools')} / {$WEBUI_NAME}
	</title>
</svelte:head>

<ImportModal
	bind:show={showImportModal}
	onImport={(tool) => {
		sessionStorage.tool = JSON.stringify({
			...tool
		});
		goto('/workspace/tools/create');
	}}
	loadUrlHandler={async (url) => {
		return await loadToolByUrl(localStorage.token, url);
	}}
	successMessage={$i18n.t('Tool imported successfully')}
/>

{#if loaded}
	<input
		id="documents-import-input"
		bind:this={toolsImportInputElement}
		bind:files={importFiles}
		type="file"
		accept=".json"
		hidden
		on:change={() => {
			console.log(importFiles);
			showConfirm = true;
		}}
	/>

	<!-- MCP & Tool Servers Section -->
	<div class="mb-6 rounded-2xl border border-gray-200/70 dark:border-gray-800/80 bg-gray-50/50 dark:bg-gray-900/30 p-4">
		<div class="flex flex-col sm:flex-row sm:items-center justify-between gap-3 mb-3">
			<div class="flex items-center gap-2.5">
				<div class="p-2 rounded-xl bg-blue-500/10 text-blue-600 dark:text-blue-400 shrink-0">
					<svg xmlns="http://www.w3.org/2000/svg" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round" class="size-5">
						<circle cx="12" cy="12" r="10" />
						<circle cx="12" cy="12" r="4" />
						<line x1="21.17" y1="8" x2="12" y2="8" />
						<line x1="3.95" y1="6.06" x2="8.54" y2="14" />
						<line x1="10.88" y1="21.94" x2="15.46" y2="14" />
					</svg>
				</div>
				<div>
					<div class="flex items-center gap-2">
						<h2 class="text-sm font-bold text-gray-900 dark:text-gray-100">
							Tool Servers & MCP
						</h2>
						<span class="text-[10px] font-semibold px-2 py-0.5 rounded-full bg-blue-100 text-blue-700 dark:bg-blue-900/50 dark:text-blue-300">
							SSE & HTTP Streamable
						</span>
					</div>
					<p class="text-xs text-gray-500 dark:text-gray-400">
						Model Context Protocol (MCP) servers (seperti Aria Page Agent, mcp-bridge-go/cf) untuk automasi browser & sistem tools.
					</p>
				</div>
			</div>

			<a
				href="/workspace/tools/create"
				class="inline-flex items-center gap-1.5 px-3 py-1.5 rounded-xl text-xs font-semibold bg-blue-600 hover:bg-blue-700 text-white transition shadow-xs cursor-pointer shrink-0"
			>
				<Plus className="size-3.5" />
				<span>Add MCP Server</span>
			</a>
		</div>

		{#if toolServers.length === 0}
			<div class="flex flex-col sm:flex-row items-center justify-between p-3.5 rounded-xl border border-dashed border-gray-200 dark:border-gray-800 bg-white/50 dark:bg-gray-850/40 gap-3 text-xs">
				<div class="space-y-1">
					<div class="font-medium text-gray-800 dark:text-gray-200">
						Belum ada MCP Server yang terhubung
					</div>
					<div class="text-gray-500 dark:text-gray-400 leading-relaxed">
						Gunakan <a href="https://github.com/kelvinzer0/aria-page-agent" target="_blank" rel="noopener noreferrer" class="text-blue-600 dark:text-blue-400 font-semibold underline">Aria Page Agent</a> untuk memberikan kemampuan browser ke AI Anda, atau hubungkan endpoint MCP apa pun dengan tipe SSE / HTTP streamable.
					</div>
				</div>
				<a
					href="/workspace/tools/create"
					class="px-3 py-1.5 text-xs font-semibold rounded-lg bg-blue-50 dark:bg-blue-950/60 text-blue-600 dark:text-blue-400 hover:bg-blue-100 dark:hover:bg-blue-900/60 border border-blue-200/50 dark:border-blue-900/50 transition shrink-0"
				>
					+ Hubungkan MCP Server
				</a>
			</div>
		{:else}
			<div class="grid gap-2">
				{#each toolServers as server}
					<div class="flex items-center justify-between p-3 rounded-xl border border-gray-200/60 dark:border-gray-800 bg-white dark:bg-gray-850 shadow-2xs">
						<div class="flex items-center gap-3 min-w-0">
							<div class="p-2 rounded-lg bg-gray-100 dark:bg-gray-800 text-gray-600 dark:text-gray-300 shrink-0">
								<WrenchAlt className="size-4" />
							</div>
							<div class="min-w-0">
								<div class="flex items-center gap-2">
									<span class="text-sm font-semibold text-gray-900 dark:text-gray-100 truncate">
										{server.info?.name || server.name || server.url}
									</span>
									<span class="text-[10px] font-bold px-1.5 py-0.5 rounded bg-blue-50 dark:bg-blue-900/40 text-blue-600 dark:text-blue-300 uppercase tracking-wider">
										{server.type === 'mcp' ? 'MCP (SSE)' : (server.type || 'MCP')}
									</span>
								</div>
								<div class="text-xs font-mono text-gray-400 dark:text-gray-500 truncate max-w-md">
									{server.url}
								</div>
							</div>
						</div>

						<div class="flex items-center gap-3 shrink-0">
							<Switch
								state={server.config?.enable ?? true}
								on:change={(e) => toggleToolServer(server, e.detail)}
							/>

							<a
								href="/workspace/tools/create?id={encodeURIComponent(server.id || server.info?.id)}"
								class="p-1.5 rounded-lg text-gray-500 hover:text-gray-900 dark:hover:text-gray-200 hover:bg-gray-100 dark:hover:bg-gray-800 transition"
								aria-label="Edit server"
							>
								<Cog6 className="size-4" />
							</a>

							<button
								type="button"
								class="p-1.5 rounded-lg text-red-500 hover:text-red-700 hover:bg-red-50 dark:hover:bg-red-950/40 transition"
								aria-label="Delete server"
								on:click={() => {
									deleteToolServer(server.id || server.info?.id);
								}}
							>
								<GarbageBin className="size-4" />
							</button>
						</div>
					</div>
				{/each}
			</div>
		{/if}
	</div>

	<div class="space-y-1">
		<div class="flex h-8 w-full items-center gap-2">
			<div class="flex min-w-0 flex-1">
				<div class=" self-center ml-1 mr-3">
					<Search className="size-3.5" />
				</div>
				<input
					class=" w-full text-sm pr-4 py-1 rounded-r-xl outline-hidden bg-transparent"
					bind:value={query}
					on:input={handleSearchInput}
					aria-label={$i18n.t('Search Tools')}
					placeholder={$i18n.t('Search Tools')}
				/>
				{#if query}
					<div class="self-center pl-1.5 translate-y-[0.5px] rounded-l-xl bg-transparent">
						<button
							class="p-0.5 rounded-full hover:bg-gray-100 dark:hover:bg-gray-900 transition"
							aria-label={$i18n.t('Clear search')}
							on:click={() => {
								query = '';
								handleSearchInput();
							}}
						>
							<XMark className="size-3" strokeWidth="2" />
						</button>
					</div>
				{/if}
			</div>

			<div
				class="flex max-w-[55%] shrink-0 overflow-x-auto scrollbar-none"
				bind:this={tagsContainerElement}
				on:wheel={(e) => {
					if (e.deltaY !== 0) {
						e.preventDefault();
						e.currentTarget.scrollLeft += e.deltaY;
					}
				}}
			>
				<div
					class="flex w-fit gap-0.5 text-center text-sm rounded-full bg-transparent whitespace-nowrap"
				>
					<ViewSelector
						bind:value={viewOption}
						align="end"
						onChange={async (value) => {
							localStorage.workspaceViewOption = value;

							await tick();
						}}
					/>
				</div>
			</div>
		</div>

		{#if (filteredItems ?? []).length !== 0}
			<div class="my-1">
				<div
					class="flex w-full items-center gap-2 px-1.5 pb-0.5 text-xs text-gray-400 dark:text-gray-600"
				>
					<button
						class="flex min-w-0 flex-1 items-center gap-1 py-0.5 text-left"
						type="button"
						on:click={() => setSortKey('name')}
					>
						{$i18n.t('Title')}
						{#if sortKey === 'name'}
							{#if sortDirection === 'asc'}
								<ChevronUp className="size-2" />
							{:else}
								<ChevronDown className="size-2" />
							{/if}
						{/if}
					</button>

					<div class="hidden w-44 shrink-0 md:block"></div>

					<button
						class="flex w-36 shrink-0 items-center justify-end gap-1 py-0.5 text-right"
						type="button"
						on:click={() => setSortKey('updated_at')}
					>
						{$i18n.t('Updated at')}
						{#if sortKey === 'updated_at'}
							{#if sortDirection === 'asc'}
								<ChevronUp className="size-2" />
							{:else}
								<ChevronDown className="size-2" />
							{/if}
						{/if}
					</button>
				</div>

				<div class="grid gap-y-0.5">
					{#each filteredItems as tool}
						<div
							class="group flex min-h-8 w-full items-center gap-2 overflow-hidden rounded-xl px-2 py-1 text-left {tool.write_access
								? 'cursor-pointer'
								: 'cursor-not-allowed opacity-60'}"
							role="button"
							tabindex={tool.write_access ? 0 : -1}
							on:click={(e) => {
								if (!tool.write_access || shouldIgnoreRowClick(e.target)) return;
								openTool(tool);
							}}
							on:keydown={(e) => {
								if (!tool.write_access || e.currentTarget !== e.target) return;
								if (e.key === 'Enter' || e.key === ' ') {
									e.preventDefault();
									openTool(tool);
								}
							}}
						>
							<div class="flex min-w-0 flex-1 items-center gap-1 overflow-hidden">
								<div class="flex min-w-0 flex-1 flex-col overflow-hidden">
									<div class="flex min-w-0 items-center gap-2 overflow-hidden">
										<div class="flex min-w-0 flex-1 items-center gap-2 overflow-hidden">
											<Tooltip content={tool.id} className="min-w-0" placement="top-start">
												<div
													class="truncate text-[0.8125rem] leading-5 text-gray-800 group-hover:underline dark:text-gray-200"
												>
													{resolveLocalizedResource(tool, $i18n.language)}
												</div>
											</Tooltip>

											{#if tool?.meta?.manifest?.version}
												<div
													class="min-w-0 max-w-[40%] shrink-0 truncate text-[0.6875rem] leading-5 text-gray-500"
												>
													v{tool?.meta?.manifest?.version ?? ''}
												</div>
											{/if}

											<Tooltip content={dayjs(tool.updated_at * 1000).format('LLLL')}>
												<div
													class="shrink-0 truncate text-[0.6875rem] leading-5 text-gray-400 dark:text-gray-600"
												>
													{dayjs(tool.updated_at * 1000).fromNow()}
												</div>
											</Tooltip>

											{#if !tool.write_access}
												<Badge type="muted" content={$i18n.t('Read Only')} />
											{/if}
										</div>
									</div>

									{#if resolveLocalizedResource(tool, $i18n.language, 'description')}
										<Tooltip
											content={resolveLocalizedResource(tool, $i18n.language, 'description')}
											className="min-w-0"
											placement="top-start"
										>
											<div
												class="mt-0.5 truncate text-[0.6875rem] leading-4 text-gray-400 dark:text-gray-600"
											>
												{resolveLocalizedResource(tool, $i18n.language, 'description')}
											</div>
										</Tooltip>
									{/if}
								</div>
							</div>

							<div
								class="hidden max-w-44 shrink-0 self-center truncate text-right text-[0.6875rem] leading-5 text-gray-500 dark:text-gray-500 md:block"
							>
								<Tooltip
									content={tool?.user?.email ?? $i18n.t('Deleted User')}
									className="min-w-0"
									placement="top-start"
								>
									<div class="truncate">
										{capitalizeFirstLetter(
											tool?.user?.name ?? tool?.user?.email ?? $i18n.t('Deleted User')
										)}
									</div>
								</Tooltip>
							</div>

							{#if tool.write_access}
								<div class="ml-2 flex shrink-0 flex-row items-center gap-1.5 self-center">
									{#if shiftKey}
										<Tooltip content={$i18n.t('Delete')}>
											<button
												class="flex size-6 items-center justify-center rounded-lg text-gray-400 transition dark:text-gray-500"
												type="button"
												aria-label={$i18n.t('Delete')}
												on:click={(e) => {
													e.preventDefault();
													e.stopPropagation();
													deleteHandler(tool);
												}}
											>
												<GarbageBin className="size-4" />
											</button>
										</Tooltip>
									{:else}
										{#if tool?.meta?.manifest?.funding_url ?? false}
											<Tooltip content="Support">
												<button
													class="flex size-6 items-center justify-center rounded-lg text-gray-400 transition dark:text-gray-500"
													type="button"
													aria-label={$i18n.t('Support')}
													on:click={(e) => {
														e.preventDefault();
														e.stopPropagation();
														selectedTool = tool;
														showManifestModal = true;
													}}
												>
													<Heart className="size-4" />
												</button>
											</Tooltip>
										{/if}

										<Tooltip content={$i18n.t('Valves')}>
											<button
												class="flex size-6 items-center justify-center rounded-lg text-gray-400 transition dark:text-gray-500"
												type="button"
												aria-label={$i18n.t('Valves')}
												on:click={(e) => {
													e.preventDefault();
													e.stopPropagation();
													selectedTool = tool;
													showValvesModal = true;
												}}
											>
												<svg
													xmlns="http://www.w3.org/2000/svg"
													fill="none"
													viewBox="0 0 24 24"
													stroke-width="1.5"
													stroke="currentColor"
													class="size-4"
												>
													<path
														stroke-linecap="round"
														stroke-linejoin="round"
														d="M9.594 3.94c.09-.542.56-.94 1.11-.94h2.593c.55 0 1.02.398 1.11.94l.213 1.281c.063.374.313.686.645.87.074.04.147.083.22.127.325.196.72.257 1.075.124l1.217-.456a1.125 1.125 0 0 1 1.37.49l1.296 2.247a1.125 1.125 0 0 1-.26 1.431l-1.003.827c-.293.241-.438.613-.43.992a7.723 7.723 0 0 1 0 .255c-.008.378.137.75.43.991l1.004.827c.424.35.534.955.26 1.43l-1.298 2.247a1.125 1.125 0 0 1-1.369.491l-1.217-.456c-.355-.133-.75-.072-1.076.124a6.47 6.47 0 0 1-.22.128c-.331.183-.581.495-.644.869l-.213 1.281c-.09.543-.56.94-1.11.94h-2.594c-.55 0-1.019-.398-1.11-.94l-.213-1.281c-.062-.374-.312-.686-.644-.87a6.52 6.52 0 0 1-.22-.127c-.325-.196-.72-.257-1.076-.124l-1.217.456a1.125 1.125 0 0 1-1.369-.49l-1.297-2.247a1.125 1.125 0 0 1 .26-1.431l1.004-.827c.292-.24.437-.613.43-.991a6.932 6.932 0 0 1 0-.255c.007-.38-.138-.751-.43-.992l-1.004-.827a1.125 1.125 0 0 1-.26-1.43l1.297-2.247a1.125 1.125 0 0 1 1.37-.491l1.216.456c.356.133.751.072 1.076-.124.072-.044.146-.086.22-.128.332-.183.582-.495.644-.869l.214-1.28Z"
													/>
													<path
														stroke-linecap="round"
														stroke-linejoin="round"
														d="M15 12a3 3 0 1 1-6 0 3 3 0 0 1 6 0Z"
													/>
												</svg>
											</button>
										</Tooltip>

										<ToolMenu
											show={openToolMenuId === tool.id}
											editHandler={() => {
												goto(`/workspace/tools/edit?id=${encodeURIComponent(tool.id)}`);
											}}
											shareHandler={() => {
												shareHandler(tool);
											}}
											cloneHandler={() => {
												cloneHandler(tool);
											}}
											exportHandler={() => {
												exportHandler(tool);
											}}
											deleteHandler={async () => {
												selectedTool = tool;
												showDeleteConfirm = true;
											}}
											onClose={() => {
												openToolMenuId = null;
											}}
										>
											<button
												class="flex size-6 items-center justify-center rounded-lg text-gray-400 transition dark:text-gray-500"
												type="button"
												aria-label={$i18n.t('Tool Menu')}
												on:click={(e) => {
													e.preventDefault();
													e.stopPropagation();
													openToolMenuId = openToolMenuId === tool.id ? null : tool.id;
												}}
											>
												<EllipsisHorizontal className="size-4" />
											</button>
										</ToolMenu>
									{/if}
								</div>
							{/if}
						</div>
					{/each}
				</div>
			</div>
		{:else}
			<div class="flex w-full flex-col items-center justify-center py-16 pb-24">
				<div class="max-w-sm text-center text-gray-900 dark:text-gray-100">
					<div class="mb-1.5 text-sm">{$i18n.t('No tools found')}</div>
					<div class="text-center text-xs leading-5 text-gray-500">
						{$i18n.t('Try adjusting your search or filter to find what you are looking for.')}
					</div>
				</div>
			</div>
		{/if}
	</div>

	{#if $config?.features.enable_community_sharing}
		<CommunityDiscover
			href="https://openwebui.com/tools"
			title={$i18n.t('Discover a tool')}
			description={$i18n.t('Discover, download, and explore custom tools')}
		/>
	{/if}

	<DeleteConfirmDialog
		bind:show={showDeleteConfirm}
		title={$i18n.t('Delete tool?')}
		on:confirm={() => {
			deleteHandler(selectedTool);
		}}
	>
		<div class=" text-sm text-gray-500 truncate">
			{$i18n.t('This will delete')}
			<span class="  font-normal">{resolveLocalizedResource(selectedTool, $i18n.language)}</span>.
		</div>
	</DeleteConfirmDialog>

	<ValvesModal
		bind:show={showValvesModal}
		type="tool"
		id={selectedTool?.id ?? null}
		meta={selectedTool?.meta}
	/>
	<ManifestModal bind:show={showManifestModal} manifest={selectedTool?.meta?.manifest ?? {}} />

	<ConfirmDialog
		bind:show={showConfirm}
		on:confirm={() => {
			const reader = new FileReader();
			reader.onload = async (event) => {
				const _tools = JSON.parse(event.target.result);
				console.log(_tools);

				for (const tool of _tools) {
					const res = await createNewTool(localStorage.token, tool).catch((error) => {
						toast.error(`${error}`);
						return null;
					});
				}

				toast.success($i18n.t('Tool imported successfully'));
				await init();
				importFiles = null;
				toolsImportInputElement.value = '';
			};

			reader.readAsText(importFiles[0]);
		}}
	>
		<div class="text-sm text-gray-500">
			<div class=" bg-yellow-500/20 text-yellow-700 dark:text-yellow-200 rounded-lg px-4 py-3">
				<div>{$i18n.t('Please carefully review the following warnings:')}</div>

				<ul class=" mt-1 list-disc pl-4 text-xs">
					<li>
						{$i18n.t('Tools have a function calling system that allows arbitrary code execution.')}.
					</li>
					<li>{$i18n.t('Do not install tools from sources you do not fully trust.')}</li>
				</ul>
			</div>

			<div class="my-3">
				{$i18n.t(
					'I acknowledge that I have read and I understand the implications of my action. I am aware of the risks associated with executing arbitrary code and I have verified the trustworthiness of the source.'
				)}
			</div>
		</div>
	</ConfirmDialog>

{:else}
	<div class="w-full h-full flex justify-center items-center">
		<Spinner className="size-5" />
	</div>
{/if}
