<script lang="ts">
	import { getContext } from 'svelte';
	import { toast } from 'svelte-sonner';
	import { goto } from '$app/navigation';
	import Modal from '$lib/components/common/Modal.svelte';
	import XMark from './icons/XMark.svelte';

	const i18n = getContext('i18n');

	export let show = false;

	const GITHUB_URL = 'https://github.com/kelvinzer0/linux-agent';
	const INSTALL_CMD = 'curl -fsSL https://raw.githubusercontent.com/kelvinzer0/linux-agent/main/install.sh | sudo bash';

	const copyToClipboard = async (text: string, label: string) => {
		try {
			await navigator.clipboard.writeText(text);
			toast.success(`${label} berhasil disalin ke clipboard!`);
		} catch (err) {
			toast.error('Gagal menyalin ke clipboard');
		}
	};
</script>

<Modal size="md" bind:show>
	<div class="flex flex-col">
		<!-- Modal Header -->
		<div class="flex justify-between items-center px-6 pt-5 pb-3 border-b border-gray-100 dark:border-gray-800">
			<div class="flex items-center gap-2.5">
				<div class="p-2 rounded-xl bg-emerald-500/10 text-emerald-600 dark:text-emerald-400">
					<svg xmlns="http://www.w3.org/2000/svg" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round" class="size-6">
						<polyline points="4 17 10 11 4 5" />
						<line x1="12" y1="19" x2="20" y2="19" />
					</svg>
				</div>
				<div>
					<div class="flex items-center gap-2">
						<h1 class="text-base font-bold text-gray-900 dark:text-gray-100">Linux Agent</h1>
						<span class="text-[10px] font-bold px-2 py-0.5 rounded-full bg-emerald-100 text-emerald-700 dark:bg-emerald-900/60 dark:text-emerald-300">
							MCP Daemon & Code Interpreter
						</span>
					</div>
					<p class="text-xs text-gray-500 dark:text-gray-400">Eksekusi Python, Bash & 18 Tools OpenCode Otomatis</p>
				</div>
			</div>

			<button
				class="text-gray-400 hover:text-gray-600 dark:hover:text-gray-200 transition p-1 rounded-lg hover:bg-gray-100 dark:hover:bg-gray-800"
				aria-label="Close"
				on:click={() => (show = false)}
			>
				<XMark className="size-5" />
			</button>
		</div>

		<!-- Modal Body -->
		<div class="px-6 py-4 space-y-4 max-h-[75vh] overflow-y-auto">
			<!-- Banner highlight -->
			<div class="p-4 rounded-xl bg-gradient-to-r from-emerald-500/10 via-teal-500/10 to-blue-500/10 border border-emerald-200/50 dark:border-emerald-800/40">
				<p class="text-xs leading-relaxed text-gray-700 dark:text-gray-300 font-medium">
					<strong>Linux Agent</strong> adalah daemon Model Context Protocol (MCP) berkinerja tinggi untuk sistem Linux yang menghubungkan AI ke mesin Anda. Dilengkapi dengan <strong>Code Interpreter</strong> (Python & Bash), 18 tools OpenCode native (<code>view</code>, <code>write</code>, <code>edit</code>, <code>patch</code>, <code>diff</code>, <code>grep</code>, <code>diagnostics</code>), serta kemampuan menjalankan otomatis <code>.agents/skills</code>.
				</p>
				<div class="mt-2.5 flex flex-wrap gap-1.5">
					<span class="text-[10px] font-mono font-medium px-2 py-0.5 rounded bg-emerald-50 dark:bg-emerald-950/80 text-emerald-600 dark:text-emerald-400 border border-emerald-200 dark:border-emerald-900">
						⚡ 18 Tools Native
					</span>
					<span class="text-[10px] font-mono font-medium px-2 py-0.5 rounded bg-blue-50 dark:bg-blue-950/80 text-blue-600 dark:text-blue-400 border border-blue-200 dark:border-blue-900">
						🐍 Python & Bash Runner
					</span>
					<span class="text-[10px] font-mono font-medium px-2 py-0.5 rounded bg-purple-50 dark:purple-950/80 text-purple-600 dark:text-purple-400 border border-purple-200 dark:border-purple-900">
						🔄 Systemd & Auto-Update
					</span>
					<span class="text-[10px] font-mono font-medium px-2 py-0.5 rounded bg-amber-50 dark:amber-950/80 text-amber-600 dark:text-amber-400 border border-amber-200 dark:border-amber-900">
						🌐 MCP SSE / Bridge
					</span>
				</div>
			</div>

			<!-- Step-by-step instructions -->
			<div>
				<h3 class="text-xs font-bold uppercase tracking-wider text-gray-500 dark:text-gray-400 mb-2.5">
					Cara Mudah Memulai:
				</h3>
				<div class="space-y-2.5 text-xs">
					<div class="flex gap-3 items-start p-2.5 rounded-lg bg-gray-50 dark:bg-gray-850/60 border border-gray-100 dark:border-gray-800">
						<div class="size-5 rounded-full bg-emerald-600 text-white flex items-center justify-center shrink-0 font-bold text-[11px]">
							1
						</div>
						<div class="space-y-1.5 flex-1 min-w-0">
							<div class="font-semibold text-gray-900 dark:text-gray-100">Install 1-Baris via Curl</div>
							<div class="text-gray-500 dark:text-gray-400">
								Jalankan perintah ini di server atau komputer Linux Anda (mendukung Ubuntu, Debian, CentOS, Arch):
							</div>
							<div class="flex items-center gap-2 p-2 bg-gray-100 dark:bg-gray-900 rounded border border-gray-200 dark:border-gray-800 font-mono text-[11px] overflow-x-auto">
								<span class="truncate flex-1 select-all">{INSTALL_CMD}</span>
								<button
									type="button"
									class="px-2 py-0.5 bg-emerald-600 hover:bg-emerald-700 text-white font-sans text-[10px] font-medium rounded shrink-0 transition"
									on:click={() => copyToClipboard(INSTALL_CMD, 'Perintah Install')}
								>
									Salin
								</button>
							</div>
						</div>
					</div>

					<div class="flex gap-3 items-start p-2.5 rounded-lg bg-gray-50 dark:bg-gray-850/60 border border-gray-100 dark:border-gray-800">
						<div class="size-5 rounded-full bg-emerald-600 text-white flex items-center justify-center shrink-0 font-bold text-[11px]">
							2
						</div>
						<div class="space-y-1">
							<div class="font-semibold text-gray-900 dark:text-gray-100">Cek Status & Ambil MCP URL</div>
							<div class="text-gray-500 dark:text-gray-400">
								Ketik <code class="px-1.5 py-0.5 rounded bg-gray-200 dark:bg-gray-800 font-mono text-[11px]">linux-agent status</code> di terminal untuk melihat status live daemon dan menyalin URL endpoint MCP Anda.
							</div>
						</div>
					</div>

					<div class="flex gap-3 items-start p-2.5 rounded-lg bg-gray-50 dark:bg-gray-850/60 border border-gray-100 dark:border-gray-800">
						<div class="size-5 rounded-full bg-emerald-600 text-white flex items-center justify-center shrink-0 font-bold text-[11px]">
							3
						</div>
						<div class="space-y-1">
							<div class="font-semibold text-gray-900 dark:text-gray-100">Hubungkan ke OpenWebUI / AI Router ZCDNS</div>
							<div class="text-gray-500 dark:text-gray-400">
								Buka <strong>Workspace > Tools > Add Connection</strong>, pilih tipe <strong>MCP (SSE / Streamable HTTP)</strong>, masukkan URL MCP Anda, dan klik <em>Verify Connection</em>. Seluruh 18 tools & eksekusi kode siap dipakai AI!
							</div>
						</div>
					</div>
				</div>
			</div>
		</div>

		<!-- Modal Footer -->
		<div class="px-6 py-3.5 bg-gray-50 dark:bg-gray-850/50 border-t border-gray-100 dark:border-gray-800 flex flex-wrap items-center justify-between gap-2">
			<div class="flex items-center gap-3">
				<button
					type="button"
					class="text-xs font-medium text-gray-600 dark:text-gray-400 hover:text-gray-900 dark:hover:text-gray-200 transition"
					on:click={() => copyToClipboard(INSTALL_CMD, 'Perintah Install')}
				>
					Salin Curl
				</button>
				<button
					type="button"
					class="text-xs font-medium text-gray-600 dark:text-gray-400 hover:text-gray-900 dark:hover:text-gray-200 transition"
					on:click={() => copyToClipboard(GITHUB_URL, 'URL GitHub')}
				>
					Salin Link Repo
				</button>
			</div>

			<div class="flex items-center gap-2">
				<a
					href={GITHUB_URL}
					target="_blank"
					rel="noopener noreferrer"
					class="inline-flex items-center gap-1.5 px-3 py-1.5 rounded-lg text-xs font-medium bg-gray-200 dark:bg-gray-700 hover:bg-gray-300 dark:hover:bg-gray-600 text-gray-800 dark:text-gray-200 transition"
				>
					<svg class="size-3.5" viewBox="0 0 24 24" fill="currentColor">
						<path d="M12 0c-6.626 0-12 5.373-12 12 0 5.302 3.438 9.8 8.207 11.387.599.111.793-.261.793-.577v-2.234c-3.338.726-4.033-1.416-4.033-1.416-.546-1.387-1.333-1.756-1.333-1.756-1.089-.745.083-.729.083-.729 1.205.084 1.839 1.237 1.839 1.237 1.07 1.834 2.807 1.304 3.492.997.107-.775.418-1.305.762-1.604-2.665-.305-5.467-1.334-5.467-5.931 0-1.311.469-2.381 1.236-3.221-.124-.303-.535-1.524.117-3.176 0 0 1.008-.322 3.301 1.23.957-.266 1.983-.399 3.003-.404 1.02.005 2.047.138 3.006.404 2.291-1.552 3.297-1.23 3.297-1.23.653 1.653.242 2.874.118 3.176.77.84 1.235 1.911 1.235 3.221 0 4.609-2.807 5.624-5.479 5.921.43.372.823 1.102.823 2.222v3.293c0 .319.192.694.801.576 4.765-1.589 8.199-6.086 8.199-11.386 0-6.627-5.373-12-12-12z" />
					</svg>
					Buka GitHub
				</a>

				<button
					type="button"
					class="inline-flex items-center gap-1.5 px-3 py-1.5 rounded-lg text-xs font-semibold bg-emerald-600 hover:bg-emerald-700 text-white transition shadow-sm"
					on:click={() => {
						show = false;
						goto('/workspace/tools');
					}}
				>
					Buka Tools OpenWebUI
				</button>
			</div>
		</div>
	</div>
</Modal>
