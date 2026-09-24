<script lang="ts">
	import { getContext } from 'svelte';

	const i18n = getContext('i18n');

	export let saveSettings: Function = () => {};

	$: currentLang = ($i18n?.language?.toLowerCase()?.startsWith('id') ||
		(typeof localStorage !== 'undefined' && localStorage.getItem('locale')?.toLowerCase()?.startsWith('id')))
		? 'id'
		: 'en';

	$: dashboardUrl = `https://zcdns.id/${currentLang}/dashboard`;
</script>

<div
	id="tab-connections"
	class="flex flex-col h-full justify-between text-sm"
>
	<h2 class="text-sm font-medium text-gray-900 dark:text-white mb-4">
		{$i18n.t('settings.personal.connections.title')}
	</h2>

	<div class="flex flex-1 min-h-0 flex-col overflow-y-auto scrollbar-hover pr-1.5 space-y-4">
		<!-- ZCDNS Dashboard Card -->
		<div class="rounded-2xl border border-blue-500/20 bg-blue-500/5 p-5 text-gray-800 dark:text-gray-200">
			<div class="flex items-start gap-3.5">
				<div class="mt-0.5 flex size-9 shrink-0 items-center justify-center rounded-xl bg-blue-500/10 text-blue-600 dark:text-blue-400">
					<svg xmlns="http://www.w3.org/2000/svg" viewBox="0 0 24 24" fill="currentColor" class="size-5">
						<path fill-rule="evenodd" d="M19.952 1.651a.75.75 0 0 1 .298.599V16.303a3 3 0 0 1-.879 2.121l-4.5 4.5a3 3 0 0 1-2.121.879H4.5a3 3 0 0 1-3-3V4.5a3 3 0 0 1 3-3h15.452ZM18.75 3.75H4.5a1.5 1.5 0 0 0-1.5 1.5v13.5a1.5 1.5 0 0 0 1.5 1.5h7.5V17.25a3 3 0 0 1 3-3h3.75V3.75Zm-2.25 15.69V16.5a1.5 1.5 0 0 0-1.5-1.5h-2.94l4.44 4.44ZM9 7.5a.75.75 0 0 1 .75.75v3a.75.75 0 0 1-1.5 0v-3A.75.75 0 0 1 9 7.5Zm6 0a.75.75 0 0 1 .75.75v3a.75.75 0 0 1-1.5 0v-3A.75.75 0 0 1 15 7.5Z" clip-rule="evenodd" />
					</svg>
				</div>
				<div class="flex-1 min-w-0">
					<h3 class="text-sm font-semibold text-gray-900 dark:text-white">
						{#if currentLang === 'id'}
							Kelola Koneksi di ZCDNS Dashboard
						{:else}
							Manage Connections in ZCDNS Dashboard
						{/if}
					</h3>
					<p class="mt-1.5 text-xs leading-relaxed text-gray-600 dark:text-gray-300">
						{#if currentLang === 'id'}
							Koneksi model AI, endpoint API, dan routing dikelola secara terpusat melalui <strong>ZCDNS Dashboard</strong>. Silakan atur endpoint dan provider AI Anda lewat dashboard.
						{:else}
							AI model connections, API endpoints, and routing are centrally managed via the <strong>ZCDNS Dashboard</strong>. Please configure your endpoints and AI providers through the dashboard.
						{/if}
					</p>

					<div class="mt-4 flex flex-wrap items-center gap-3">
						<a
							href={dashboardUrl}
							target="_blank"
							rel="noopener noreferrer"
							class="inline-flex items-center gap-2 rounded-xl bg-blue-600 px-4 py-2 text-xs font-medium text-white shadow-sm transition hover:bg-blue-500 focus:outline-none focus:ring-2 focus:ring-blue-500/50"
						>
							{#if currentLang === 'id'}
								Buka Dashboard ZCDNS
							{:else}
								Open ZCDNS Dashboard
							{/if}
							<svg xmlns="http://www.w3.org/2000/svg" viewBox="0 0 20 20" fill="currentColor" class="size-3.5">
								<path fill-rule="evenodd" d="M4.25 5.5a.75.75 0 0 0-.75.75v8.5c0 .414.336.75.75.75h8.5a.75.75 0 0 0 .75-.75v-4a.75.75 0 0 1 1.5 0v4A2.25 2.25 0 0 1 12.75 17h-8.5A2.25 2.25 0 0 1 2 14.75v-8.5A2.25 2.25 0 0 1 4.25 4h4a.75.75 0 0 1 0 1.5h-4Z" clip-rule="evenodd" />
								<path fill-rule="evenodd" d="M6.194 12.753a.75.75 0 0 0 1.06.053L16.5 4.44v2.81a.75.75 0 0 0 1.5 0v-4.5a.75.75 0 0 0-.75-.75h-4.5a.75.75 0 0 0 0 1.5h2.553l-9.056 8.194a.75.75 0 0 0-.053 1.06Z" clip-rule="evenodd" />
							</svg>
						</a>
						<span class="text-xs text-gray-500 dark:text-gray-400 font-mono select-all">
							{dashboardUrl}
						</span>
					</div>
				</div>
			</div>
		</div>

		<!-- Direct Link Info Box -->
		<div class="rounded-xl border border-gray-100 bg-gray-50/50 p-4 text-xs text-gray-500 dark:border-white/5 dark:bg-white/[0.02] dark:text-gray-400 space-y-2">
			<div class="font-medium text-gray-700 dark:text-gray-300">
				{#if currentLang === 'id'}
					Informasi AI Router & Endpoint
				{:else}
					AI Router & Endpoint Information
				{/if}
			</div>
			<p>
				{#if currentLang === 'id'}
					Semua request chat ke model yang terdaftar di workspace ini secara otomatis diarahkan melalui ZCDNS AI Router proxy. Anda tidak perlu menambahkan custom endpoint secara manual di browser.
				{:else}
					All chat completions for models registered in this workspace are automatically routed through the ZCDNS AI Router proxy. You do not need to configure custom endpoints manually in the browser.
				{/if}
			</p>
		</div>
	</div>
</div>
