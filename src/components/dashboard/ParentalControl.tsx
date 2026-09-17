import { useState, useEffect } from 'react';
import {
  Shield,
  ShieldAlert,
  ShieldCheck,
  Smartphone,
  Laptop,
  Wifi,
  Globe,
  Plus,
  Copy,
  Check,
  Zap,
  Save,
} from 'lucide-react';
import { Button } from '../ui/button';
import { Input } from '../ui/input';
import type { ParentalConfig, TestQueryResult } from './types';
import { dashboardApi } from './api';

interface Props {
  subdomain: string;
  baseDomain: string;
  dnsPort: number;
}

export function ParentalControl({ subdomain, baseDomain, dnsPort }: Props) {
  const [config, setConfig] = useState<ParentalConfig | null>(null);
  const [isLoading, setIsLoading] = useState(true);
  const [isSaving, setIsSaving] = useState(false);
  const [copiedDoH, setCopiedDoH] = useState(false);
  const [copiedDot, setCopiedDot] = useState(false);

  // New domain inputs
  const [newBlocked, setNewBlocked] = useState('');
  const [newAllowed, setNewAllowed] = useState('');

  // Live tester inside parental control
  const [testDomain, setTestDomain] = useState('pornhub.com');
  const [testResult, setTestResult] = useState<TestQueryResult | null>(null);
  const [isTesting, setIsTesting] = useState(false);

  // Setup tab state: 'android' | 'browser' | 'ios' | 'router'
  const [setupTab, setSetupTab] = useState<'android' | 'browser' | 'ios' | 'router'>('android');

  const fullDomain = `${subdomain}.${baseDomain}`;
  const dohUrl = `https://${baseDomain}/dns-query/${subdomain}`;

  useEffect(() => {
    loadConfig();
  }, [subdomain]);

  const loadConfig = async () => {
    setIsLoading(true);
    try {
      const data = await dashboardApi.getParentalConfig(subdomain);
      setConfig(data);
    } catch (err) {
      console.error('Failed to load parental config:', err);
    } finally {
      setIsLoading(false);
    }
  };

  const handleToggle = (key: keyof ParentalConfig) => {
    if (!config) return;
    setConfig({
      ...config,
      [key]: !config[key],
    });
  };

  const handleSave = async () => {
    if (!config) return;
    setIsSaving(true);
    try {
      const saved = await dashboardApi.saveParentalConfig(subdomain, config);
      setConfig(saved);
      alert('Pengaturan Parental Control berhasil disimpan!');
    } catch (err: any) {
      alert(err.message || 'Gagal menyimpan pengaturan');
    } finally {
      setIsSaving(false);
    }
  };

  const handleAddBlocked = (e: React.FormEvent) => {
    e.preventDefault();
    if (!config || !newBlocked.trim()) return;
    const clean = newBlocked.trim().toLowerCase().replace(/^https?:\/\//, '').replace(/\/.*$/, '');
    if (!config.custom_blocked.includes(clean)) {
      setConfig({
        ...config,
        custom_blocked: [...config.custom_blocked, clean],
      });
    }
    setNewBlocked('');
  };

  const handleRemoveBlocked = (domain: string) => {
    if (!config) return;
    setConfig({
      ...config,
      custom_blocked: config.custom_blocked.filter((d) => d !== domain),
    });
  };

  const handleAddAllowed = (e: React.FormEvent) => {
    e.preventDefault();
    if (!config || !newAllowed.trim()) return;
    const clean = newAllowed.trim().toLowerCase().replace(/^https?:\/\//, '').replace(/\/.*$/, '');
    if (!config.custom_allowed.includes(clean)) {
      setConfig({
        ...config,
        custom_allowed: [...config.custom_allowed, clean],
      });
    }
    setNewAllowed('');
  };

  const handleRemoveAllowed = (domain: string) => {
    if (!config) return;
    setConfig({
      ...config,
      custom_allowed: config.custom_allowed.filter((d) => d !== domain),
    });
  };

  const handleRunTest = async () => {
    if (!testDomain.trim()) return;
    setIsTesting(true);
    try {
      // First save current config so the engine tests against latest rules
      if (config) {
        await dashboardApi.saveParentalConfig(subdomain, config);
      }
      const res = await dashboardApi.testQuery(testDomain.trim(), 'A');
      setTestResult(res);
    } catch (err: any) {
      console.error(err);
    } finally {
      setIsTesting(false);
    }
  };

  const copyText = (text: string, setFn: (v: boolean) => void) => {
    navigator.clipboard.writeText(text);
    setFn(true);
    setTimeout(() => setFn(false), 2000);
  };

  if (isLoading || !config) {
    return (
      <div className="p-12 text-center text-gray-500 text-sm">
        Memuat konfigurasi Parental Control & Web Blocker...
      </div>
    );
  }

  return (
    <div className="space-y-8">
      {/* Master Status Card */}
      <div className="bg-white rounded-2xl border border-gray-200 shadow-sm p-6">
        <div className="flex flex-col lg:flex-row lg:items-center lg:justify-between gap-6">
          <div className="flex items-start space-x-4">
            <div
              className={`w-12 h-12 rounded-2xl flex items-center justify-center font-bold flex-shrink-0 ${
                config.enabled ? 'bg-green-100 text-green-700' : 'bg-gray-100 text-gray-400'
              }`}
            >
              {config.enabled ? <ShieldCheck className="w-7 h-7" /> : <ShieldAlert className="w-7 h-7" />}
            </div>
            <div>
              <div className="flex items-center space-x-2">
                <h2 className="text-xl font-bold text-gray-900">Parental Control & Web Blocker</h2>
                <span
                  className={`text-xs font-semibold px-2.5 py-0.5 rounded-full ${
                    config.enabled ? 'bg-green-100 text-green-800' : 'bg-gray-100 text-gray-600'
                  }`}
                >
                  {config.enabled ? 'AKTIF' : 'NONAKTIF'}
                </span>
              </div>
              <p className="text-sm text-gray-600 mt-1 max-w-2xl">
                Subdomain <strong>{fullDomain}</strong> Anda dapat digunakan sebagai DNS Resolver penyaring konten berbahaya, situs pornografi, judi online, iklan, dan pembatasan media sosial pada HP/laptop keluarga.
              </p>
            </div>
          </div>

          <div className="flex items-center space-x-4 self-end lg:self-center">
            <button
              onClick={() => handleToggle('enabled')}
              className={`relative inline-flex h-7 w-14 flex-shrink-0 cursor-pointer rounded-full border-2 border-transparent transition-colors duration-200 ease-in-out focus:outline-none ${
                config.enabled ? 'bg-green-600' : 'bg-gray-300'
              }`}
            >
              <span
                className={`pointer-events-none inline-block h-6 w-6 transform rounded-full bg-white shadow-md ring-0 transition duration-200 ease-in-out ${
                  config.enabled ? 'translate-x-7' : 'translate-x-0'
                }`}
              />
            </button>
            <Button
              onClick={handleSave}
              disabled={isSaving}
              className="bg-[#012241] hover:bg-[#02365f] text-white flex items-center space-x-2 px-5"
            >
              <Save className="w-4 h-4" />
              <span>{isSaving ? 'Menyimpan...' : 'Simpan Perubahan'}</span>
            </Button>
          </div>
        </div>

        {/* DNS Endpoint Details */}
        <div className="mt-6 pt-5 border-t border-gray-100 grid grid-cols-1 md:grid-cols-2 gap-4">
          <div className="bg-gray-50 p-3.5 rounded-xl border border-gray-200 flex items-center justify-between">
            <div>
              <span className="text-[11px] font-semibold text-gray-500 uppercase block">
                Private DNS Hostname (Android / TLS)
              </span>
              <code className="text-sm font-bold text-gray-900 font-mono">{fullDomain}</code>
            </div>
            <button
              onClick={() => copyText(fullDomain, setCopiedDot)}
              className="p-2 text-gray-500 hover:text-green-700 rounded-lg hover:bg-gray-200 transition-colors"
              title="Salin Hostname"
            >
              {copiedDot ? <Check className="w-4 h-4 text-green-600" /> : <Copy className="w-4 h-4" />}
            </button>
          </div>

          <div className="bg-gray-50 p-3.5 rounded-xl border border-gray-200 flex items-center justify-between">
            <div>
              <span className="text-[11px] font-semibold text-gray-500 uppercase block">
                DNS-over-HTTPS (DoH) URL (Browser / iOS)
              </span>
              <code className="text-sm font-bold text-gray-900 font-mono truncate max-w-xs block">
                {dohUrl}
              </code>
            </div>
            <button
              onClick={() => copyText(dohUrl, setCopiedDoH)}
              className="p-2 text-gray-500 hover:text-green-700 rounded-lg hover:bg-gray-200 transition-colors"
              title="Salin URL DoH"
            >
              {copiedDoH ? <Check className="w-4 h-4 text-green-600" /> : <Copy className="w-4 h-4" />}
            </button>
          </div>
        </div>
      </div>

      {/* Categories Protection Grid */}
      <div className="bg-white rounded-2xl border border-gray-200 shadow-sm p-6 space-y-6">
        <div>
          <h3 className="text-lg font-bold text-gray-900 flex items-center space-x-2">
            <Shield className="w-5 h-5 text-green-600" />
            <span>Kategori Pemblokiran Otomatis</span>
          </h3>
          <p className="text-xs text-gray-500 mt-0.5">
            Aktifkan kategori perlindungan yang ingin diblokir secara otomatis oleh DNS resolver Anda
          </p>
        </div>

        <div className="grid grid-cols-1 md:grid-cols-2 lg:grid-cols-3 gap-4">
          {/* Adult */}
          <div
            onClick={() => handleToggle('block_adult')}
            className={`p-4 rounded-xl border-2 cursor-pointer transition-all ${
              config.block_adult
                ? 'border-green-600 bg-green-50/30'
                : 'border-gray-200 hover:border-gray-300 bg-white'
            }`}
          >
            <div className="flex items-center justify-between mb-2">
              <span className="text-2xl">🔞</span>
              <span
                className={`text-xs font-bold px-2 py-0.5 rounded ${
                  config.block_adult ? 'bg-green-600 text-white' : 'bg-gray-200 text-gray-600'
                }`}
              >
                {config.block_adult ? 'DIBLOKIR' : 'DIIJINKAN'}
              </span>
            </div>
            <h4 className="font-bold text-sm text-gray-900">Konten Dewasa & Pornografi (18+)</h4>
            <p className="text-xs text-gray-500 mt-1">
              Blokir situs pornografi, konten dewasa eksplisit, dan situs tidak ramah anak.
            </p>
          </div>

          {/* Gambling */}
          <div
            onClick={() => handleToggle('block_gambling')}
            className={`p-4 rounded-xl border-2 cursor-pointer transition-all ${
              config.block_gambling
                ? 'border-green-600 bg-green-50/30'
                : 'border-gray-200 hover:border-gray-300 bg-white'
            }`}
          >
            <div className="flex items-center justify-between mb-2">
              <span className="text-2xl">🎰</span>
              <span
                className={`text-xs font-bold px-2 py-0.5 rounded ${
                  config.block_gambling ? 'bg-green-600 text-white' : 'bg-gray-200 text-gray-600'
                }`}
              >
                {config.block_gambling ? 'DIBLOKIR' : 'DIIJINKAN'}
              </span>
            </div>
            <h4 className="font-bold text-sm text-gray-900">Judi Online & Slot Gacor</h4>
            <p className="text-xs text-gray-500 mt-1">
              Blokir situs taruhan, agen slot, togel online, casino, dan platform perjudian.
            </p>
          </div>

          {/* Malware & Phishing */}
          <div
            onClick={() => handleToggle('block_malware')}
            className={`p-4 rounded-xl border-2 cursor-pointer transition-all ${
              config.block_malware
                ? 'border-green-600 bg-green-50/30'
                : 'border-gray-200 hover:border-gray-300 bg-white'
            }`}
          >
            <div className="flex items-center justify-between mb-2">
              <span className="text-2xl">🛡️</span>
              <span
                className={`text-xs font-bold px-2 py-0.5 rounded ${
                  config.block_malware ? 'bg-green-600 text-white' : 'bg-gray-200 text-gray-600'
                }`}
              >
                {config.block_malware ? 'DIBLOKIR' : 'DIIJINKAN'}
              </span>
            </div>
            <h4 className="font-bold text-sm text-gray-900">Malware, Scam & Phishing</h4>
            <p className="text-xs text-gray-500 mt-1">
              Lindungi gawai dari serangan pencurian data, link phising penipuan, dan virus.
            </p>
          </div>

          {/* Ads & Trackers */}
          <div
            onClick={() => handleToggle('block_ads')}
            className={`p-4 rounded-xl border-2 cursor-pointer transition-all ${
              config.block_ads
                ? 'border-green-600 bg-green-50/30'
                : 'border-gray-200 hover:border-gray-300 bg-white'
            }`}
          >
            <div className="flex items-center justify-between mb-2">
              <span className="text-2xl">📢</span>
              <span
                className={`text-xs font-bold px-2 py-0.5 rounded ${
                  config.block_ads ? 'bg-green-600 text-white' : 'bg-gray-200 text-gray-600'
                }`}
              >
                {config.block_ads ? 'DIBLOKIR' : 'DIIJINKAN'}
              </span>
            </div>
            <h4 className="font-bold text-sm text-gray-900">Iklan & Pelacak Data (AdBlock)</h4>
            <p className="text-xs text-gray-500 mt-1">
              Blokir iklan pop-up yang mengganggu dan pelacak pengumpul data privasi di web.
            </p>
          </div>

          {/* Social Media */}
          <div
            onClick={() => handleToggle('block_social')}
            className={`p-4 rounded-xl border-2 cursor-pointer transition-all ${
              config.block_social
                ? 'border-green-600 bg-green-50/30'
                : 'border-gray-200 hover:border-gray-300 bg-white'
            }`}
          >
            <div className="flex items-center justify-between mb-2">
              <span className="text-2xl">📱</span>
              <span
                className={`text-xs font-bold px-2 py-0.5 rounded ${
                  config.block_social ? 'bg-green-600 text-white' : 'bg-gray-200 text-gray-600'
                }`}
              >
                {config.block_social ? 'DIBLOKIR' : 'DIIJINKAN'}
              </span>
            </div>
            <h4 className="font-bold text-sm text-gray-900">Media Sosial (TikTok, IG, X)</h4>
            <p className="text-xs text-gray-500 mt-1">
              Batasi akses anak ke TikTok, Instagram, Facebook, dan platform media sosial.
            </p>
          </div>

          {/* Gaming Platforms */}
          <div
            onClick={() => handleToggle('block_gaming')}
            className={`p-4 rounded-xl border-2 cursor-pointer transition-all ${
              config.block_gaming
                ? 'border-green-600 bg-green-50/30'
                : 'border-gray-200 hover:border-gray-300 bg-white'
            }`}
          >
            <div className="flex items-center justify-between mb-2">
              <span className="text-2xl">🎮</span>
              <span
                className={`text-xs font-bold px-2 py-0.5 rounded ${
                  config.block_gaming ? 'bg-green-600 text-white' : 'bg-gray-200 text-gray-600'
                }`}
              >
                {config.block_gaming ? 'DIBLOKIR' : 'DIIJINKAN'}
              </span>
            </div>
            <h4 className="font-bold text-sm text-gray-900">Game Online (Roblox, Steam)</h4>
            <p className="text-xs text-gray-500 mt-1">
              Kendalikan waktu bermain game online seperti Roblox, Steam, dan Discord saat jam belajar.
            </p>
          </div>
        </div>

        {/* SafeSearch & Block Mode settings */}
        <div className="pt-4 border-t border-gray-100 grid grid-cols-1 sm:grid-cols-2 gap-4">
          <div className="flex items-center justify-between p-4 rounded-xl bg-gray-50 border border-gray-200">
            <div>
              <h4 className="font-bold text-sm text-gray-900">Paksa SafeSearch</h4>
              <p className="text-xs text-gray-500">
                Kunci pencarian aman otomatis di Google, YouTube, Bing, dan DuckDuckGo.
              </p>
            </div>
            <button
              onClick={() => handleToggle('enforce_safesearch')}
              className={`relative inline-flex h-6 w-11 flex-shrink-0 cursor-pointer rounded-full border-2 border-transparent transition-colors duration-200 ease-in-out ${
                config.enforce_safesearch ? 'bg-green-600' : 'bg-gray-300'
              }`}
            >
              <span
                className={`pointer-events-none inline-block h-5 w-5 transform rounded-full bg-white shadow-md ring-0 transition duration-200 ease-in-out ${
                  config.enforce_safesearch ? 'translate-x-5' : 'translate-x-0'
                }`}
              />
            </button>
          </div>

          <div className="flex items-center justify-between p-4 rounded-xl bg-gray-50 border border-gray-200">
            <div>
              <h4 className="font-bold text-sm text-gray-900">Metode Blokir (Sinkhole)</h4>
              <p className="text-xs text-gray-500">
                Respon balik saat perangkat membuka domain terblokir.
              </p>
            </div>
            <select
              value={config.block_mode}
              onChange={(e) => setConfig({ ...config, block_mode: e.target.value as '0.0.0.0' | 'NXDOMAIN' })}
              className="h-9 px-3 border border-gray-300 rounded-lg text-xs bg-white font-semibold text-gray-800"
            >
              <option value="0.0.0.0">0.0.0.0 (Sinkhole IP)</option>
              <option value="NXDOMAIN">NXDOMAIN (Domain Tidak Ada)</option>
            </select>
          </div>
        </div>
      </div>

      {/* Custom Blocklist & Allowlist */}
      <div className="grid grid-cols-1 lg:grid-cols-2 gap-6">
        {/* Custom Blocklist */}
        <div className="bg-white rounded-2xl border border-gray-200 shadow-sm p-6 space-y-4">
          <div className="flex items-center justify-between">
            <h3 className="font-bold text-base text-gray-900 flex items-center space-x-2">
              <span className="text-red-500 font-bold">🚫</span>
              <span>Daftar Blokir Kustom (Blacklist)</span>
            </h3>
            <span className="text-xs bg-red-50 text-red-700 font-semibold px-2 py-0.5 rounded-full">
              {config.custom_blocked.length}
            </span>
          </div>
          <p className="text-xs text-gray-500">
            Tambahkan nama domain spesifik yang ingin Anda blokir secara manual
          </p>

          <form onSubmit={handleAddBlocked} className="flex gap-2">
            <Input
              type="text"
              placeholder="contoh: roblox.com atau reddit.com"
              value={newBlocked}
              onChange={(e) => setNewBlocked(e.target.value)}
              className="h-9 text-xs font-mono"
            />
            <Button type="submit" size="sm" className="bg-red-600 hover:bg-red-700 text-white h-9 px-4">
              <Plus className="w-4 h-4 mr-1" />
              Blokir
            </Button>
          </form>

          <div className="flex flex-wrap gap-2 max-h-48 overflow-y-auto pt-2">
            {config.custom_blocked.length === 0 ? (
              <span className="text-xs text-gray-400 italic">Belum ada domain di daftar blokir kustom.</span>
            ) : (
              config.custom_blocked.map((dom) => (
                <span
                  key={dom}
                  className="inline-flex items-center space-x-1.5 px-2.5 py-1 rounded-lg bg-red-50 text-red-800 border border-red-200 text-xs font-mono"
                >
                  <span>{dom}</span>
                  <button
                    onClick={() => handleRemoveBlocked(dom)}
                    className="text-red-400 hover:text-red-700 ml-1 cursor-pointer"
                  >
                    ×
                  </button>
                </span>
              ))
            )}
          </div>
        </div>

        {/* Custom Allowlist */}
        <div className="bg-white rounded-2xl border border-gray-200 shadow-sm p-6 space-y-4">
          <div className="flex items-center justify-between">
            <h3 className="font-bold text-base text-gray-900 flex items-center space-x-2">
              <span className="text-green-500 font-bold">✅</span>
              <span>Daftar Pengecualian (Whitelist)</span>
            </h3>
            <span className="text-xs bg-green-50 text-green-700 font-semibold px-2 py-0.5 rounded-full">
              {config.custom_allowed.length}
            </span>
          </div>
          <p className="text-xs text-gray-500">
            Domain yang selalu diijinkan dibuka meskipun masuk ke dalam kategori blokir
          </p>

          <form onSubmit={handleAddAllowed} className="flex gap-2">
            <Input
              type="text"
              placeholder="contoh: wikipedia.org"
              value={newAllowed}
              onChange={(e) => setNewAllowed(e.target.value)}
              className="h-9 text-xs font-mono"
            />
            <Button type="submit" size="sm" className="bg-green-600 hover:bg-green-700 text-white h-9 px-4">
              <Plus className="w-4 h-4 mr-1" />
              Ijinkan
            </Button>
          </form>

          <div className="flex flex-wrap gap-2 max-h-48 overflow-y-auto pt-2">
            {config.custom_allowed.length === 0 ? (
              <span className="text-xs text-gray-400 italic">Belum ada domain di daftar pengecualian.</span>
            ) : (
              config.custom_allowed.map((dom) => (
                <span
                  key={dom}
                  className="inline-flex items-center space-x-1.5 px-2.5 py-1 rounded-lg bg-green-50 text-green-800 border border-green-200 text-xs font-mono"
                >
                  <span>{dom}</span>
                  <button
                    onClick={() => handleRemoveAllowed(dom)}
                    className="text-green-400 hover:text-green-700 ml-1 cursor-pointer"
                  >
                    ×
                  </button>
                </span>
              ))
            )}
          </div>
        </div>
      </div>

      {/* Live Parental Test Tool */}
      <div className="bg-white rounded-2xl border border-gray-200 shadow-sm p-6 space-y-4">
        <h3 className="font-bold text-base text-gray-900 flex items-center space-x-2">
          <Zap className="w-4 h-4 text-amber-500" />
          <span>Uji Coba Pemblokiran Langsung (Instant Test)</span>
        </h3>
        <p className="text-xs text-gray-500">
          Uji coba query domain langsung di peramban untuk melihat bagaimana DNS Parental Control merespon
        </p>

        <div className="flex flex-col sm:flex-row gap-3">
          <Input
            type="text"
            placeholder="Ketik domain misal: pornhub.com, tiktok.com, google.com"
            value={testDomain}
            onChange={(e) => setTestDomain(e.target.value)}
            className="h-10 text-sm font-mono"
          />
          <Button
            onClick={handleRunTest}
            disabled={isTesting}
            className="bg-[#012241] hover:bg-[#02365f] text-white px-6 font-medium h-10"
          >
            {isTesting ? 'Menguji...' : 'Uji Blokir'}
          </Button>
        </div>

        {testResult && (
          <div
            className={`p-4 rounded-xl border text-xs font-mono space-y-2 ${
              testResult.answers && testResult.answers.some((a) => a.includes('0.0.0.0')) ||
              testResult.rcode === 'NXDOMAIN'
                ? 'bg-red-50 border-red-200 text-red-950'
                : 'bg-green-50 border-green-200 text-green-950'
            }`}
          >
            <div className="flex items-center justify-between font-bold">
              <span>
                {testResult.answers && testResult.answers.some((a) => a.includes('0.0.0.0')) ||
                testResult.rcode === 'NXDOMAIN'
                  ? '🛑 HASIL: DOMAIN BERHASIL DIBLOKIR / SINKHOLED'
                  : '✅ HASIL: DOMAIN DIIJINKAN (FORWARDED KE UPSTREAM)'}
              </span>
              <span>Status: {testResult.rcode} ({testResult.response_time_ms} ms)</span>
            </div>
            <div>
              Answers:{' '}
              {testResult.answers && testResult.answers.length > 0 ? (
                testResult.answers.join(', ')
              ) : (
                <span className="italic">Kosong (NXDOMAIN)</span>
              )}
            </div>
          </div>
        )}
      </div>

      {/* Device Setup Instructions */}
      <div className="bg-white rounded-2xl border border-gray-200 shadow-sm p-6 space-y-6">
        <div>
          <h3 className="text-lg font-bold text-gray-900 flex items-center space-x-2">
            <Globe className="w-5 h-5 text-blue-600" />
            <span>Cara Memasang di Perangkat (Setup Guide)</span>
          </h3>
          <p className="text-xs text-gray-500 mt-0.5">
            Pilih jenis perangkat untuk melihat petunjuk mengaktifkan DNS Parental Control Anda
          </p>
        </div>

        <div className="flex items-center space-x-2 border-b border-gray-200 pb-2">
          <button
            onClick={() => setSetupTab('android')}
            className={`flex items-center space-x-1.5 px-3 py-1.5 rounded-lg text-xs font-bold transition-colors ${
              setupTab === 'android' ? 'bg-[#012241] text-white' : 'text-gray-600 hover:bg-gray-100'
            }`}
          >
            <Smartphone className="w-3.5 h-3.5" />
            <span>Android</span>
          </button>
          <button
            onClick={() => setSetupTab('browser')}
            className={`flex items-center space-x-1.5 px-3 py-1.5 rounded-lg text-xs font-bold transition-colors ${
              setupTab === 'browser' ? 'bg-[#012241] text-white' : 'text-gray-600 hover:bg-gray-100'
            }`}
          >
            <Laptop className="w-3.5 h-3.5" />
            <span>Browser (Chrome / Edge / Firefox)</span>
          </button>
          <button
            onClick={() => setSetupTab('ios')}
            className={`flex items-center space-x-1.5 px-3 py-1.5 rounded-lg text-xs font-bold transition-colors ${
              setupTab === 'ios' ? 'bg-[#012241] text-white' : 'text-gray-600 hover:bg-gray-100'
            }`}
          >
            <span>🍎 iOS / macOS</span>
          </button>
          <button
            onClick={() => setSetupTab('router')}
            className={`flex items-center space-x-1.5 px-3 py-1.5 rounded-lg text-xs font-bold transition-colors ${
              setupTab === 'router' ? 'bg-[#012241] text-white' : 'text-gray-600 hover:bg-gray-100'
            }`}
          >
            <Wifi className="w-3.5 h-3.5" />
            <span>Router Rumah (WiFi)</span>
          </button>
        </div>

        <div className="text-xs text-gray-700 leading-relaxed bg-gray-50 p-5 rounded-xl border border-gray-200">
          {setupTab === 'android' && (
            <div className="space-y-3">
              <h4 className="font-bold text-sm text-gray-900">Langkah Pengaturan di Android (Tanpa Aplikasi):</h4>
              <ol className="list-decimal list-inside space-y-1.5 font-medium text-gray-700">
                <li>Buka menu <strong>Setelan (Settings)</strong> di HP Android.</li>
                <li>Pilih <strong>Koneksi & Berbagi (Connections & Sharing)</strong> atau <strong>Jaringan & Internet</strong>.</li>
                <li>Pilih <strong>DNS Pribadi (Private DNS)</strong>.</li>
                <li>Pilih opsi <strong>Nama host penyedia DNS pribadi (Private DNS provider hostname)</strong>.</li>
                <li>
                  Ketik nama domain sandbox Anda:{' '}
                  <code className="bg-white px-2 py-0.5 rounded border border-gray-300 font-bold text-green-700">
                    {fullDomain}
                  </code>
                </li>
                <li>Klik <strong>Simpan (Save)</strong>. Semua browser dan aplikasi di HP anak kini otomatis terlindungi!</li>
              </ol>
            </div>
          )}

          {setupTab === 'browser' && (
            <div className="space-y-3">
              <h4 className="font-bold text-sm text-gray-900">Langkah Pengaturan di Google Chrome / Brave / Edge:</h4>
              <ol className="list-decimal list-inside space-y-1.5 font-medium text-gray-700">
                <li>Buka Browser, klik titik tiga di pojok kanan atas lalu pilih <strong>Settings (Setelan)</strong>.</li>
                <li>Pilih menu <strong>Privacy and Security (Privasi dan Keamanan)</strong> &gt; <strong>Security</strong>.</li>
                <li>Gulir ke bawah ke bagian <strong>Use Secure DNS (Gunakan DNS Aman)</strong>.</li>
                <li>Pilih opsi <strong>With (Kustom / Custom)</strong>.</li>
                <li>
                  Tempel URL DNS-over-HTTPS Anda:{' '}
                  <code className="bg-white px-2 py-0.5 rounded border border-gray-300 font-bold text-green-700">
                    {dohUrl}
                  </code>
                </li>
                <li>Selesai! Browser sekarang menggunakan DNS Parental Control Anda.</li>
              </ol>
            </div>
          )}

          {setupTab === 'ios' && (
            <div className="space-y-3">
              <h4 className="font-bold text-sm text-gray-900">Langkah Pengaturan di iPhone / iPad / Mac:</h4>
              <p>
                Gunakan aplikasi penyedia profil DNS gratis (seperti DNSCloak atau Apple Configurator Profile) dan masukkan DoH URL Anda:
              </p>
              <code className="block bg-white p-2.5 rounded border border-gray-300 font-bold text-green-700 font-mono">
                {dohUrl}
              </code>
            </div>
          )}

          {setupTab === 'router' && (
            <div className="space-y-3">
              <h4 className="font-bold text-sm text-gray-900">Langkah Pengaturan di Router WiFi Rumah:</h4>
              <p>
                Dengan mengarahkan DNS di router, seluruh smart TV, HP, tablet, dan laptop yang terhubung ke WiFi rumah otomatis terlindungi tanpa perlu mengatur satu per satu.
              </p>
              <ol className="list-decimal list-inside space-y-1 font-medium text-gray-700">
                <li>Buka dashboard admin router Anda (biasanya di <code>192.168.1.1</code>).</li>
                <li>Cari menu <strong>DHCP Server</strong> atau <strong>WAN / Network DNS</strong>.</li>
                <li>Masukkan alamat IP DNS Server ZeroCentDNS (port <code>{dnsPort}</code>) ke kolom <strong>Primary DNS</strong>.</li>
                <li>Simpan dan restart router.</li>
              </ol>
            </div>
          )}
        </div>
      </div>
    </div>
  );
}
