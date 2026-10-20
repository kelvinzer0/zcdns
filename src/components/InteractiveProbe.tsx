import { useState } from 'react';
import { Terminal, Send, Radio, Shield, Zap, ArrowRight, CheckCircle2 } from 'lucide-react';
import { LocalizedLink as Link } from './LocalizedLink';

export function InteractiveProbe() {
  const [activeTab, setActiveTab] = useState<'dig' | 'stream' | 'adblock'>('dig');
  const [queryInput, setQueryInput] = useState('demo.zcdns.id');
  const [queryType, setQueryType] = useState('A');
  const [isQuerying, setIsQuerying] = useState(false);
  const [response, setResponse] = useState<{
    status: string;
    server: string;
    timeMs: number;
    answers: Array<{ name: string; type: string; value: string; ttl: number }>;
  }>({
    status: 'NOERROR',
    server: 'ns1.zcdns.id (Anycast Edge)',
    timeMs: 14,
    answers: [
      { name: 'demo.zcdns.id.', type: 'A', value: '192.0.2.1', ttl: 60 },
      { name: 'demo.zcdns.id.', type: 'TXT', value: '"ZeroCentDNS Sandbox - Free DNS Education"', ttl: 60 },
    ],
  });

  const handleTestRun = () => {
    setIsQuerying(true);
    setTimeout(() => {
      setResponse({
        status: 'NOERROR',
        server: 'ns1.zcdns.id (Anycast Edge)',
        timeMs: Math.floor(Math.random() * 20) + 8,
        answers: [
          {
            name: `${queryInput.endsWith('.') ? queryInput : queryInput + '.'}`,
            type: queryType,
            value: queryType === 'AAAA' ? '2001:db8::1' : queryType === 'CNAME' ? 'upstream.example.com.' : '192.0.2.1',
            ttl: 60,
          },
        ],
      });
      setIsQuerying(false);
    }, 350);
  };

  return (
    <div className="w-full max-w-4xl mx-auto rounded-xl border border-slate-800 bg-[#0c121e] shadow-2xl overflow-hidden text-left" data-ui="dns-interactive-probe">
      {/* Probe Window Header */}
      <div className="bg-[#131b2e] px-4 py-3 border-b border-slate-800 flex items-center justify-between">
        <div className="flex items-center space-x-2">
          <div className="w-3 h-3 rounded-full bg-red-500/80" />
          <div className="w-3 h-3 rounded-full bg-amber-500/80" />
          <div className="w-3 h-3 rounded-full bg-emerald-500/80" />
          <span className="ml-2 font-mono text-xs text-slate-400 font-medium hidden sm:inline">
            zcdns-live-playground — ns1.zcdns.id:53
          </span>
        </div>
        <div className="flex items-center space-x-1">
          <button
            type="button"
            onClick={() => setActiveTab('dig')}
            className={`px-3 py-1 rounded text-xs font-mono font-medium transition-colors ${
              activeTab === 'dig' ? 'bg-blue-600 text-white' : 'text-slate-400 hover:text-slate-200 hover:bg-slate-800'
            }`}
          >
            <span className="flex items-center gap-1.5">
              <Terminal className="w-3.5 h-3.5" />
              Web Dig
            </span>
          </button>
          <button
            type="button"
            onClick={() => setActiveTab('stream')}
            className={`px-3 py-1 rounded text-xs font-mono font-medium transition-colors ${
              activeTab === 'stream' ? 'bg-blue-600 text-white' : 'text-slate-400 hover:text-slate-200 hover:bg-slate-800'
            }`}
          >
            <span className="flex items-center gap-1.5">
              <Radio className="w-3.5 h-3.5 text-emerald-400 animate-pulse" />
              Live SSE Stream
            </span>
          </button>
          <button
            type="button"
            onClick={() => setActiveTab('adblock')}
            className={`px-3 py-1 rounded text-xs font-mono font-medium transition-colors ${
              activeTab === 'adblock' ? 'bg-blue-600 text-white' : 'text-slate-400 hover:text-slate-200 hover:bg-slate-800'
            }`}
          >
            <span className="flex items-center gap-1.5">
              <Shield className="w-3.5 h-3.5 text-purple-400" />
              Resolver & AdBlock
            </span>
          </button>
        </div>
      </div>

      {/* Tab 1: Web Dig Simulator */}
      {activeTab === 'dig' && (
        <div className="p-4 sm:p-6 space-y-4">
          <div className="flex flex-col sm:flex-row gap-2">
            <div className="relative flex-1">
              <span className="absolute left-3 top-2.5 font-mono text-xs text-emerald-500 font-semibold">$ dig @ns1</span>
              <input
                type="text"
                value={queryInput}
                onChange={(e) => setQueryInput(e.target.value)}
                className="w-full bg-[#070b13] border border-slate-700/80 rounded pl-24 pr-3 py-2 text-xs font-mono text-slate-100 focus:outline-none focus:border-blue-500"
                placeholder="domain.zcdns.id"
              />
            </div>
            <select
              value={queryType}
              onChange={(e) => setQueryType(e.target.value)}
              className="bg-[#070b13] border border-slate-700/80 rounded px-3 py-2 text-xs font-mono text-slate-100 focus:outline-none focus:border-blue-500"
            >
              <option value="A">A</option>
              <option value="AAAA">AAAA</option>
              <option value="CNAME">CNAME</option>
              <option value="TXT">TXT</option>
              <option value="MX">MX</option>
            </select>
            <button
              type="button"
              onClick={handleTestRun}
              disabled={isQuerying}
              className="px-4 py-2 bg-emerald-600 hover:bg-emerald-500 disabled:opacity-60 text-white rounded text-xs font-mono font-medium flex items-center justify-center gap-1.5 transition-colors"
            >
              <Send className="w-3.5 h-3.5" />
              {isQuerying ? 'Menguji...' : 'Kirim Kueri'}
            </button>
          </div>

          {/* Response Output */}
          <div className="bg-[#070b13] border border-slate-800/80 rounded p-4 font-mono text-xs text-slate-300 space-y-2">
            <div className="flex items-center justify-between text-slate-500 border-b border-slate-800/60 pb-2">
              <span className="flex items-center gap-1.5 text-emerald-400">
                <CheckCircle2 className="w-3.5 h-3.5" /> status: {response.status}
              </span>
              <span>latency: <strong className="text-slate-300">{response.timeMs} ms</strong></span>
              <span className="hidden sm:inline">server: {response.server}</span>
            </div>
            <div className="pt-1 space-y-1">
              <p className="text-slate-500">;; ANSWER SECTION:</p>
              {response.answers.map((ans, idx) => (
                <div key={idx} className="flex flex-wrap items-center gap-x-4 gap-y-1 text-slate-200">
                  <span className="text-blue-400 font-semibold">{ans.name}</span>
                  <span className="text-slate-500">{ans.ttl}</span>
                  <span className="text-slate-500">IN</span>
                  <span className="px-1.5 py-0.5 rounded bg-blue-950 text-blue-300 font-bold text-[10px]">{ans.type}</span>
                  <span className="text-amber-300">{ans.value}</span>
                </div>
              ))}
            </div>
          </div>
        </div>
      )}

      {/* Tab 2: Live SSE Stream Simulator */}
      {activeTab === 'stream' && (
        <div className="p-4 sm:p-6 space-y-3">
          <div className="flex items-center justify-between text-xs font-mono text-slate-400 border-b border-slate-800 pb-2">
            <div className="flex items-center gap-2">
              <span className="w-2 h-2 rounded-full bg-emerald-500 animate-ping" />
              <span className="text-emerald-400 font-semibold">SSE Connected: eventsource active</span>
            </div>
            <span>Listening UDP:53 / DoH</span>
          </div>
          <div className="bg-[#070b13] border border-slate-800 rounded p-3 font-mono text-xs space-y-2">
            <div className="flex items-center justify-between text-slate-300 py-1 border-b border-slate-800/40">
              <span className="text-slate-500">10:54:12.102</span>
              <span className="text-cyan-400">203.0.113.45</span>
              <span className="text-amber-400 font-bold">IN A</span>
              <span className="text-slate-200">fox91.zcdns.id</span>
              <span className="px-1.5 py-0.5 text-[10px] bg-emerald-950 text-emerald-300 border border-emerald-800">NOERROR</span>
            </div>
            <div className="flex items-center justify-between text-slate-300 py-1 border-b border-slate-800/40">
              <span className="text-slate-500">10:54:14.881</span>
              <span className="text-cyan-400">198.51.100.22</span>
              <span className="text-purple-400 font-bold">IN TXT</span>
              <span className="text-slate-200">_acme-challenge.fox91.zcdns.id</span>
              <span className="px-1.5 py-0.5 text-[10px] bg-emerald-950 text-emerald-300 border border-emerald-800">NOERROR</span>
            </div>
            <div className="flex items-center justify-between text-slate-300 py-1">
              <span className="text-slate-500">10:54:19.450</span>
              <span className="text-cyan-400">103.24.56.12</span>
              <span className="text-blue-400 font-bold">IN AAAA</span>
              <span className="text-slate-200">api.fox91.zcdns.id</span>
              <span className="px-1.5 py-0.5 text-[10px] bg-emerald-950 text-emerald-300 border border-emerald-800">NOERROR</span>
            </div>
          </div>
          <p className="text-xs text-slate-400">
            Setiap kueri DNS yang menjangkau nameserver kami dialirkan seketika ke browser Anda tanpa polling.
          </p>
        </div>
      )}

      {/* Tab 3: Resolver & AdBlock */}
      {activeTab === 'adblock' && (
        <div className="p-4 sm:p-6 space-y-4">
          <div className="grid grid-cols-1 sm:grid-cols-2 gap-3 text-xs font-mono">
            <div className="p-3 bg-[#070b13] border border-slate-800 rounded">
              <div className="text-slate-400 mb-1">Encrypted DNS (DoT / DoH)</div>
              <div className="text-emerald-400 font-semibold break-all">tls://dns.zcdns.id</div>
              <div className="text-[11px] text-slate-500 mt-1">Dapat langsung dipasang di Android Private DNS & iOS profile.</div>
            </div>
            <div className="p-3 bg-[#070b13] border border-slate-800 rounded">
              <div className="text-slate-400 mb-1">Penyaring Konten & Iklan</div>
              <div className="text-purple-400 font-semibold">Aktif • 2 Ancaman Diblokir</div>
              <div className="text-[11px] text-slate-500 mt-1">Otomatis meredam pelacak telemetri dan malware.</div>
            </div>
          </div>
          <p className="text-xs text-slate-400">
            Gunakan subdomain Anda sebagai DNS resolver privat dengan rule filter custom dan kontrol keluarga.
          </p>
        </div>
      )}

      {/* Footer Strip */}
      <div className="bg-[#080d17] px-4 py-3 border-t border-slate-800 flex flex-col sm:flex-row items-center justify-between gap-3 text-xs">
        <span className="text-slate-400 flex items-center gap-1.5">
          <Zap className="w-3.5 h-3.5 text-amber-400" />
          Klaim subdomain terisolasi Anda dalam 1-klik tanpa kartu kredit
        </span>
        <Link
          to="/dashboard"
          className="inline-flex items-center gap-1.5 text-blue-400 hover:text-blue-300 font-medium transition-colors"
        >
          Buka Dasbor Penuh
          <ArrowRight className="w-3.5 h-3.5" />
        </Link>
      </div>
    </div>
  );
}
