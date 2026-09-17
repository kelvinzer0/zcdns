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
import { useTranslations } from '../../lib/useTranslations';

interface Props {
  subdomain: string;
  baseDomain: string;
  dnsPort?: number;
}

export function ParentalControl({ subdomain, baseDomain }: Props) {
  const t = useTranslations('Dashboard');
  const [config, setConfig] = useState<ParentalConfig | null>(null);
  const [isLoading, setIsLoading] = useState(true);
  const [isSaving, setIsSaving] = useState(false);
  const [copiedDoH, setCopiedDoH] = useState(false);
  const [copiedDot, setCopiedDot] = useState(false);

  // New domain inputs
  const [newBlocked, setNewBlocked] = useState('');
  const [newAllowed, setNewAllowed] = useState('');

  // Live tester inside parental control
  const [testDomain, setTestDomain] = useState('roblox.com');
  const [testResult, setTestResult] = useState<TestQueryResult | null>(null);
  const [isTesting, setIsTesting] = useState(false);

  // Setup tab state: 'android' | 'browser' | 'ios' | 'router'
  const [setupTab, setSetupTab] = useState<'android' | 'browser' | 'ios' | 'router'>('android');

  const guardDomain = `${subdomain}.guard.${baseDomain}`;
  const dohUrl = `https://${baseDomain}/dns-query/guard/${subdomain}`;

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
      alert(t('parental-save-success'));
    } catch (err: any) {
      alert(err.message || t('parental-save-failed'));
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
      if (config) {
        await dashboardApi.saveParentalConfig(subdomain, config);
      }
      const res = await dashboardApi.testQuery(testDomain.trim(), 'A', undefined, subdomain);
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
      <div className="p-12 text-center text-gray-500 text-sm font-mono">
        {t('loading')}
      </div>
    );
  }

  return (
    <div className="space-y-6">
      {/* Master Status Card */}
      <div className="bg-white border border-gray-200 p-4 sm:p-6 space-y-4 rounded-none">
        <div className="flex flex-col sm:flex-row sm:items-center sm:justify-between gap-4">
          <div className="flex items-start space-x-3">
            <div
              className={`w-10 h-10 flex items-center justify-center font-bold flex-shrink-0 rounded-none border ${
                config.enabled ? 'bg-green-50 text-green-700 border-green-200' : 'bg-gray-50 text-gray-400 border-gray-200'
              }`}
            >
              {config.enabled ? <ShieldCheck className="w-6 h-6" /> : <ShieldAlert className="w-6 h-6" />}
            </div>
            <div>
              <div className="flex items-center space-x-2">
                <h2 className="text-base sm:text-lg font-bold text-gray-900">{t('parental-title')}</h2>
                <span
                  className={`text-[10px] font-bold px-1.5 py-0.5 rounded-none border uppercase ${
                    config.enabled ? 'bg-green-50 text-green-800 border-green-200' : 'bg-gray-50 text-gray-600 border-gray-200'
                  }`}
                >
                  {config.enabled ? t('parental-status-active') : t('parental-status-inactive')}
                </span>
              </div>
              <p className="text-xs text-gray-600 mt-1 max-w-2xl">
                {t('parental-desc')}
              </p>
            </div>
          </div>

          <div className="flex items-center space-x-3 self-end sm:self-center">
            <button
              type="button"
              onClick={() => handleToggle('enabled')}
              className={`relative inline-flex h-6 w-12 flex-shrink-0 cursor-pointer rounded-none border transition-colors ${
                config.enabled ? 'bg-green-600 border-green-700' : 'bg-gray-200 border-gray-300'
              }`}
            >
              <span
                className={`pointer-events-none inline-block h-5 w-5 transform rounded-none bg-white shadow-xs transition duration-150 ${
                  config.enabled ? 'translate-x-6' : 'translate-x-0'
                }`}
              />
            </button>
            <Button
              onClick={handleSave}
              disabled={isSaving}
              className="bg-[#012241] hover:bg-[#02365f] text-white flex items-center space-x-1.5 px-3.5 h-8 text-xs font-semibold rounded-none"
            >
              <Save className="w-3.5 h-3.5" />
              <span>{isSaving ? t('parental-btn-saving') : t('parental-btn-save')}</span>
            </Button>
          </div>
        </div>

        {/* DNS Endpoint Details */}
        <div className="pt-3 border-t border-gray-200 grid grid-cols-1 md:grid-cols-2 gap-3">
          <div className="bg-gray-50 p-3 border border-gray-200 rounded-none flex flex-col justify-between">
            <div className="flex items-center justify-between">
              <div className="overflow-hidden">
                <span className="text-[10px] font-semibold text-gray-500 uppercase block">
                  Private DNS Hostname (Android / TLS)
                </span>
                <code className="text-xs sm:text-sm font-bold text-gray-900 font-mono truncate block">{guardDomain}</code>
              </div>
              <button
                type="button"
                onClick={() => copyText(guardDomain, setCopiedDot)}
                className="p-1.5 text-gray-500 hover:text-green-700 rounded-none hover:bg-gray-200 transition-colors flex-shrink-0 ml-2"
                title="Salin Hostname"
              >
                {copiedDot ? <Check className="w-4 h-4 text-green-600" /> : <Copy className="w-4 h-4" />}
              </button>
            </div>
            <p className="mt-2 text-[11px] text-green-800 bg-green-50 border border-green-200 px-2 py-1 leading-snug">
              ✅ {t('parental-dot-ipv6-note')}
            </p>
          </div>

          <div className="bg-gray-50 p-3 border border-gray-200 rounded-none flex items-center justify-between">
            <div className="overflow-hidden">
              <span className="text-[10px] font-semibold text-gray-500 uppercase block">
                DoH URL (Browser / iOS)
              </span>
              <code className="text-xs sm:text-sm font-bold text-gray-900 font-mono truncate block">
                {dohUrl}
              </code>
            </div>
            <button
              type="button"
              onClick={() => copyText(dohUrl, setCopiedDoH)}
              className="p-1.5 text-gray-500 hover:text-green-700 rounded-none hover:bg-gray-200 transition-colors flex-shrink-0 ml-2"
              title="Salin URL DoH"
            >
              {copiedDoH ? <Check className="w-4 h-4 text-green-600" /> : <Copy className="w-4 h-4" />}
            </button>
          </div>
        </div>
      </div>

      {/* Categories Protection Grid */}
      <div className="bg-white border border-gray-200 p-4 sm:p-6 space-y-4 rounded-none">
        <div>
          <h3 className="text-base font-bold text-gray-900 flex items-center space-x-2">
            <Shield className="w-4 h-4 text-green-600" />
            <span>{t('parental-cat-title')}</span>
          </h3>
          <p className="text-xs text-gray-500 mt-0.5">
            {t('parental-cat-desc')}
          </p>
        </div>

        <div className="grid grid-cols-1 sm:grid-cols-2 lg:grid-cols-3 gap-3">
          {/* Adult */}
          <div
            onClick={() => handleToggle('block_adult')}
            className={`p-3.5 border cursor-pointer select-none transition-all rounded-none ${
              config.block_adult
                ? 'border-green-600 bg-green-50/25'
                : 'border-gray-200 hover:border-gray-300 bg-white'
            }`}
          >
            <div className="flex items-center justify-between mb-1.5">
              <span className="text-xl">🔞</span>
              <span
                className={`text-[10px] font-bold px-1.5 py-0.5 uppercase rounded-none border ${
                  config.block_adult ? 'bg-green-600 text-white border-green-600' : 'bg-gray-100 text-gray-600 border-gray-300'
                }`}
              >
                {config.block_adult ? t('parental-badge-block') : t('parental-badge-allow')}
              </span>
            </div>
            <h4 className="font-bold text-xs sm:text-sm text-gray-900">{t('parental-adult-title')}</h4>
            <p className="text-[11px] text-gray-500 mt-0.5">{t('parental-adult-desc')}</p>
          </div>

          {/* Gambling */}
          <div
            onClick={() => handleToggle('block_gambling')}
            className={`p-3.5 border cursor-pointer select-none transition-all rounded-none ${
              config.block_gambling
                ? 'border-green-600 bg-green-50/25'
                : 'border-gray-200 hover:border-gray-300 bg-white'
            }`}
          >
            <div className="flex items-center justify-between mb-1.5">
              <span className="text-xl">🎰</span>
              <span
                className={`text-[10px] font-bold px-1.5 py-0.5 uppercase rounded-none border ${
                  config.block_gambling ? 'bg-green-600 text-white border-green-600' : 'bg-gray-100 text-gray-600 border-gray-300'
                }`}
              >
                {config.block_gambling ? t('parental-badge-block') : t('parental-badge-allow')}
              </span>
            </div>
            <h4 className="font-bold text-xs sm:text-sm text-gray-900">{t('parental-gambling-title')}</h4>
            <p className="text-[11px] text-gray-500 mt-0.5">{t('parental-gambling-desc')}</p>
          </div>

          {/* Malware & Phishing */}
          <div
            onClick={() => handleToggle('block_malware')}
            className={`p-3.5 border cursor-pointer select-none transition-all rounded-none ${
              config.block_malware
                ? 'border-green-600 bg-green-50/25'
                : 'border-gray-200 hover:border-gray-300 bg-white'
            }`}
          >
            <div className="flex items-center justify-between mb-1.5">
              <span className="text-xl">🛡️</span>
              <span
                className={`text-[10px] font-bold px-1.5 py-0.5 uppercase rounded-none border ${
                  config.block_malware ? 'bg-green-600 text-white border-green-600' : 'bg-gray-100 text-gray-600 border-gray-300'
                }`}
              >
                {config.block_malware ? t('parental-badge-block') : t('parental-badge-allow')}
              </span>
            </div>
            <h4 className="font-bold text-xs sm:text-sm text-gray-900">{t('parental-malware-title')}</h4>
            <p className="text-[11px] text-gray-500 mt-0.5">{t('parental-malware-desc')}</p>
          </div>

          {/* Ads & Trackers */}
          <div
            onClick={() => handleToggle('block_ads')}
            className={`p-3.5 border cursor-pointer select-none transition-all rounded-none ${
              config.block_ads
                ? 'border-green-600 bg-green-50/25'
                : 'border-gray-200 hover:border-gray-300 bg-white'
            }`}
          >
            <div className="flex items-center justify-between mb-1.5">
              <span className="text-xl">📢</span>
              <span
                className={`text-[10px] font-bold px-1.5 py-0.5 uppercase rounded-none border ${
                  config.block_ads ? 'bg-green-600 text-white border-green-600' : 'bg-gray-100 text-gray-600 border-gray-300'
                }`}
              >
                {config.block_ads ? t('parental-badge-block') : t('parental-badge-allow')}
              </span>
            </div>
            <h4 className="font-bold text-xs sm:text-sm text-gray-900">{t('parental-ads-title')}</h4>
            <p className="text-[11px] text-gray-500 mt-0.5">{t('parental-ads-desc')}</p>
          </div>

          {/* Social Media */}
          <div
            onClick={() => handleToggle('block_social')}
            className={`p-3.5 border cursor-pointer select-none transition-all rounded-none ${
              config.block_social
                ? 'border-green-600 bg-green-50/25'
                : 'border-gray-200 hover:border-gray-300 bg-white'
            }`}
          >
            <div className="flex items-center justify-between mb-1.5">
              <span className="text-xl">📱</span>
              <span
                className={`text-[10px] font-bold px-1.5 py-0.5 uppercase rounded-none border ${
                  config.block_social ? 'bg-green-600 text-white border-green-600' : 'bg-gray-100 text-gray-600 border-gray-300'
                }`}
              >
                {config.block_social ? t('parental-badge-block') : t('parental-badge-allow')}
              </span>
            </div>
            <h4 className="font-bold text-xs sm:text-sm text-gray-900">{t('parental-social-title')}</h4>
            <p className="text-[11px] text-gray-500 mt-0.5">{t('parental-social-desc')}</p>
          </div>

          {/* Gaming Platforms */}
          <div
            onClick={() => handleToggle('block_gaming')}
            className={`p-3.5 border cursor-pointer select-none transition-all rounded-none ${
              config.block_gaming
                ? 'border-green-600 bg-green-50/25'
                : 'border-gray-200 hover:border-gray-300 bg-white'
            }`}
          >
            <div className="flex items-center justify-between mb-1.5">
              <span className="text-xl">🎮</span>
              <span
                className={`text-[10px] font-bold px-1.5 py-0.5 uppercase rounded-none border ${
                  config.block_gaming ? 'bg-green-600 text-white border-green-600' : 'bg-gray-100 text-gray-600 border-gray-300'
                }`}
              >
                {config.block_gaming ? t('parental-badge-block') : t('parental-badge-allow')}
              </span>
            </div>
            <h4 className="font-bold text-xs sm:text-sm text-gray-900">{t('parental-gaming-title')}</h4>
            <p className="text-[11px] text-gray-500 mt-0.5">{t('parental-gaming-desc')}</p>
          </div>
        </div>

        {/* SafeSearch & Block Mode settings */}
        <div className="pt-3 border-t border-gray-200 grid grid-cols-1 sm:grid-cols-2 gap-3">
          <div className="flex items-center justify-between p-3 rounded-none bg-gray-50 border border-gray-200">
            <div>
              <h4 className="font-bold text-xs sm:text-sm text-gray-900">{t('parental-safesearch-title')}</h4>
              <p className="text-[11px] text-gray-500">
                {t('parental-safesearch-desc')}
              </p>
            </div>
            <button
              type="button"
              onClick={() => handleToggle('enforce_safesearch')}
              className={`relative inline-flex h-5 w-10 flex-shrink-0 cursor-pointer rounded-none border transition-colors ${
                config.enforce_safesearch ? 'bg-green-600 border-green-700' : 'bg-gray-200 border-gray-300'
              }`}
            >
              <span
                className={`pointer-events-none inline-block h-4 w-4 transform rounded-none bg-white shadow-xs transition duration-150 ${
                  config.enforce_safesearch ? 'translate-x-5' : 'translate-x-0'
                }`}
              />
            </button>
          </div>

          <div className="flex items-center justify-between p-3 rounded-none bg-gray-50 border border-gray-200">
            <div>
              <h4 className="font-bold text-xs sm:text-sm text-gray-900">{t('parental-sinkhole-title')}</h4>
              <p className="text-[11px] text-gray-500">
                {t('parental-sinkhole-desc')}
              </p>
            </div>
            <select
              value={config.block_mode}
              onChange={(e) => setConfig({ ...config, block_mode: e.target.value as '0.0.0.0' | 'NXDOMAIN' })}
              className="h-8 px-2 border border-gray-300 rounded-none text-xs bg-white font-semibold text-gray-800"
            >
              <option value="0.0.0.0">0.0.0.0</option>
              <option value="NXDOMAIN">NXDOMAIN</option>
            </select>
          </div>
        </div>
      </div>

      {/* Custom Blocklist & Allowlist */}
      <div className="grid grid-cols-1 lg:grid-cols-2 gap-4">
        {/* Custom Blocklist */}
        <div className="bg-white border border-gray-200 p-4 sm:p-5 space-y-3 rounded-none">
          <div className="flex items-center justify-between">
            <h3 className="font-bold text-sm sm:text-base text-gray-900 flex items-center space-x-1.5">
              <span>🚫</span>
              <span>{t('parental-custom-block-title')}</span>
            </h3>
            <span className="text-xs bg-red-50 text-red-700 font-semibold px-2 py-0.5 border border-red-200 rounded-none">
              {config.custom_blocked.length}
            </span>
          </div>
          <p className="text-xs text-gray-500">
            {t('parental-custom-block-desc')}
          </p>

          <form onSubmit={handleAddBlocked} className="flex gap-2">
            <Input
              type="text"
              placeholder={t('parental-custom-block-placeholder')}
              value={newBlocked}
              onChange={(e) => setNewBlocked(e.target.value)}
              className="h-8 text-xs font-mono rounded-none"
            />
            <Button type="submit" size="sm" className="bg-red-600 hover:bg-red-700 text-white h-8 px-3 rounded-none text-xs font-semibold">
              <Plus className="w-3.5 h-3.5 mr-1" />
              {t('parental-badge-block')}
            </Button>
          </form>

          <div className="flex flex-wrap gap-1.5 max-h-40 overflow-y-auto pt-1">
            {config.custom_blocked.length === 0 ? (
              <span className="text-xs text-gray-400 italic">{t('parental-empty-blacklist')}</span>
            ) : (
              config.custom_blocked.map((dom) => (
                <span
                  key={dom}
                  className="inline-flex items-center space-x-1.5 px-2 py-0.5 bg-red-50 text-red-800 border border-red-200 text-xs font-mono rounded-none"
                >
                  <span>{dom}</span>
                  <button
                    type="button"
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
        <div className="bg-white border border-gray-200 p-4 sm:p-5 space-y-3 rounded-none">
          <div className="flex items-center justify-between">
            <h3 className="font-bold text-sm sm:text-base text-gray-900 flex items-center space-x-1.5">
              <span>✅</span>
              <span>{t('parental-custom-allow-title')}</span>
            </h3>
            <span className="text-xs bg-green-50 text-green-700 font-semibold px-2 py-0.5 border border-green-200 rounded-none">
              {config.custom_allowed.length}
            </span>
          </div>
          <p className="text-xs text-gray-500">
            {t('parental-custom-allow-desc')}
          </p>

          <form onSubmit={handleAddAllowed} className="flex gap-2">
            <Input
              type="text"
              placeholder={t('parental-custom-allow-placeholder')}
              value={newAllowed}
              onChange={(e) => setNewAllowed(e.target.value)}
              className="h-8 text-xs font-mono rounded-none"
            />
            <Button type="submit" size="sm" className="bg-green-600 hover:bg-green-700 text-white h-8 px-3 rounded-none text-xs font-semibold">
              <Plus className="w-3.5 h-3.5 mr-1" />
              {t('parental-badge-allow')}
            </Button>
          </form>

          <div className="flex flex-wrap gap-1.5 max-h-40 overflow-y-auto pt-1">
            {config.custom_allowed.length === 0 ? (
              <span className="text-xs text-gray-400 italic">{t('parental-empty-whitelist')}</span>
            ) : (
              config.custom_allowed.map((dom) => (
                <span
                  key={dom}
                  className="inline-flex items-center space-x-1.5 px-2 py-0.5 bg-green-50 text-green-800 border border-green-200 text-xs font-mono rounded-none"
                >
                  <span>{dom}</span>
                  <button
                    type="button"
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
      <div className="bg-white border border-gray-200 p-4 sm:p-5 space-y-3 rounded-none">
        <h3 className="font-bold text-sm sm:text-base text-gray-900 flex items-center space-x-2">
          <Zap className="w-4 h-4 text-amber-500" />
          <span>{t('parental-test-title')}</span>
        </h3>
        <p className="text-xs text-gray-500">
          {t('parental-test-desc')}
        </p>

        <div className="flex flex-col sm:flex-row gap-2">
          <Input
            type="text"
            placeholder={t('parental-test-placeholder')}
            value={testDomain}
            onChange={(e) => setTestDomain(e.target.value)}
            className="h-9 text-xs sm:text-sm font-mono rounded-none"
          />
          <Button
            onClick={handleRunTest}
            disabled={isTesting}
            className="bg-[#012241] hover:bg-[#02365f] text-white px-5 font-medium h-9 rounded-none text-xs whitespace-nowrap"
          >
            {isTesting ? t('parental-test-testing') : t('parental-test-btn')}
          </Button>
        </div>

        {/* Quick Presets */}
        <div className="flex flex-wrap gap-1.5 items-center pt-1">
          <span className="text-[11px] text-gray-500 font-semibold mr-1 font-mono">Uji Cepat:</span>
          {[
            { label: 'Roblox (Game)', domain: 'roblox.com' },
            { label: 'Pornhub (Adult)', domain: 'pornhub.com' },
            { label: 'DoubleClick (Ads)', domain: 'doubleclick.net' },
            { label: 'TikTok (Social)', domain: 'tiktok.com' },
            { label: 'Google (SafeSearch)', domain: 'google.com' },
          ].map((preset) => (
            <button
              key={preset.domain}
              type="button"
              onClick={() => setTestDomain(preset.domain)}
              className={`px-2 py-0.5 text-xs font-mono border cursor-pointer transition-colors ${
                testDomain === preset.domain
                  ? 'bg-blue-50 text-blue-700 border-blue-300 font-bold'
                  : 'bg-gray-50 text-gray-700 border-gray-200 hover:bg-gray-100'
              }`}
            >
              {preset.label}
            </button>
          ))}
        </div>

        {testResult && (
          <div
            className={`p-3 border text-xs font-mono space-y-1.5 rounded-none ${
              (testResult.answers && testResult.answers.some((a) => a.includes('0.0.0.0') || a.includes('::') || a.includes('216.239.38.') || a.includes('204.79.197.'))) ||
              testResult.rcode === 'NXDOMAIN'
                ? 'bg-red-50 border-red-200 text-red-950'
                : 'bg-green-50 border-green-200 text-green-950'
            }`}
          >
            <div className="flex items-center justify-between font-bold">
              <span>
                {(testResult.answers && testResult.answers.some((a) => a.includes('0.0.0.0') || a.includes('::') || a.includes('216.239.38.') || a.includes('204.79.197.'))) ||
                testResult.rcode === 'NXDOMAIN'
                  ? `🛑 ${t('parental-test-blocked')}`
                  : `✅ ${t('parental-test-allowed')}`}
              </span>
              <span>{testResult.rcode} ({testResult.response_time_ms} ms)</span>
            </div>
            <div>
              Answers:{' '}
              {testResult.answers && testResult.answers.length > 0 ? (
                testResult.answers.join(', ')
              ) : (
                <span className="italic">{t('parental-test-empty-answers')}</span>
              )}
            </div>
            {testResult.server && (
              <div className="text-[11px] text-gray-500 pt-1 border-t border-gray-200/50">
                Server: {testResult.server}
              </div>
            )}
          </div>
        )}
      </div>

      {/* Device Setup Instructions */}
      <div className="bg-white border border-gray-200 p-4 sm:p-5 space-y-4 rounded-none">
        <div>
          <h3 className="text-base font-bold text-gray-900 flex items-center space-x-2">
            <Globe className="w-4 h-4 text-blue-600" />
            <span>{t('parental-setup-title')}</span>
          </h3>
          <p className="text-xs text-gray-500 mt-0.5">
            {t('parental-setup-desc')}
          </p>
        </div>

        <div className="flex items-center space-x-1 border-b border-gray-200 pb-0 overflow-x-auto scrollbar-none">
          <button
            type="button"
            onClick={() => setSetupTab('android')}
            className={`flex items-center space-x-1.5 px-3 py-2 text-xs font-bold transition-colors rounded-none border-b-2 whitespace-nowrap ${
              setupTab === 'android' ? 'border-[#012241] text-[#012241] bg-gray-50' : 'border-transparent text-gray-600 hover:text-gray-900'
            }`}
          >
            <Smartphone className="w-3.5 h-3.5" />
            <span>{t('parental-tab-android')}</span>
          </button>
          <button
            type="button"
            onClick={() => setSetupTab('browser')}
            className={`flex items-center space-x-1.5 px-3 py-2 text-xs font-bold transition-colors rounded-none border-b-2 whitespace-nowrap ${
              setupTab === 'browser' ? 'border-[#012241] text-[#012241] bg-gray-50' : 'border-transparent text-gray-600 hover:text-gray-900'
            }`}
          >
            <Laptop className="w-3.5 h-3.5" />
            <span>{t('parental-tab-browser')}</span>
          </button>
          <button
            type="button"
            onClick={() => setSetupTab('ios')}
            className={`flex items-center space-x-1.5 px-3 py-2 text-xs font-bold transition-colors rounded-none border-b-2 whitespace-nowrap ${
              setupTab === 'ios' ? 'border-[#012241] text-[#012241] bg-gray-50' : 'border-transparent text-gray-600 hover:text-gray-900'
            }`}
          >
            <span>{t('parental-tab-ios')}</span>
          </button>
          <button
            type="button"
            onClick={() => setSetupTab('router')}
            className={`flex items-center space-x-1.5 px-3 py-2 text-xs font-bold transition-colors rounded-none border-b-2 whitespace-nowrap ${
              setupTab === 'router' ? 'border-[#012241] text-[#012241] bg-gray-50' : 'border-transparent text-gray-600 hover:text-gray-900'
            }`}
          >
            <Wifi className="w-3.5 h-3.5" />
            <span>{t('parental-tab-router')}</span>
          </button>
        </div>

        <div className="text-xs text-gray-700 leading-relaxed bg-gray-50 p-4 border border-gray-200 rounded-none">
          {setupTab === 'android' && (
            <div className="space-y-2">
              <h4 className="font-bold text-xs sm:text-sm text-gray-900">{t('parental-setup-android-title')}</h4>
              <ol className="list-decimal list-inside space-y-1 font-medium text-gray-700">
                <li>{t('parental-setup-android-1')}</li>
                <li>{t('parental-setup-android-2')}</li>
                <li>{t('parental-setup-android-3')}</li>
                <li>
                  {t('parental-setup-android-4')}{' '}
                  <code className="bg-white px-1.5 py-0.5 rounded-none border border-gray-300 font-bold text-green-700 font-mono">
                    {guardDomain}
                  </code>
                </li>
                <li>{t('parental-setup-android-5')}</li>
              </ol>
              <p className="text-[11px] text-green-800 bg-green-50 border border-green-200 px-2 py-1.5 leading-snug">
                ✅ {t('parental-dot-ipv6-note')}
              </p>
            </div>
          )}

          {setupTab === 'browser' && (
            <div className="space-y-2">
              <h4 className="font-bold text-xs sm:text-sm text-gray-900">{t('parental-setup-browser-title')}</h4>
              <ol className="list-decimal list-inside space-y-1 font-medium text-gray-700">
                <li>{t('parental-setup-browser-1')}</li>
                <li>{t('parental-setup-browser-2')}</li>
                <li>{t('parental-setup-browser-3')}</li>
                <li>
                  {t('parental-setup-browser-4')}{' '}
                  <code className="bg-white px-1.5 py-0.5 rounded-none border border-gray-300 font-bold text-green-700 font-mono break-all">
                    {dohUrl}
                  </code>
                </li>
              </ol>
            </div>
          )}

          {setupTab === 'ios' && (
            <div className="space-y-2">
              <h4 className="font-bold text-xs sm:text-sm text-gray-900">{t('parental-setup-ios-title')}</h4>
              <p>
                {t('parental-setup-ios-desc')}
              </p>
              <code className="block bg-white p-2 rounded-none border border-gray-300 font-bold text-green-700 font-mono break-all">
                {dohUrl}
              </code>
            </div>
          )}

          {setupTab === 'router' && (
            <div className="space-y-2">
              <h4 className="font-bold text-xs sm:text-sm text-gray-900">{t('parental-setup-router-title')}</h4>
              <ol className="list-decimal list-inside space-y-1 font-medium text-gray-700">
                <li>{t('parental-setup-router-1')}</li>
                <li>{t('parental-setup-router-2')}</li>
                <li>{t('parental-setup-router-3')}</li>
                <li>{t('parental-setup-router-4')}</li>
              </ol>
            </div>
          )}
        </div>
      </div>
    </div>
  );
}
