import React, { useState, useEffect, useCallback } from 'react';
import {
  Network, Plus, Trash2, Save, Copy, Check, Eye, EyeOff,
  Loader2, ChevronDown, ChevronRight, AlertCircle, CheckCircle2,
  Key, ShieldCheck, ExternalLink, MessageSquare
} from 'lucide-react';
import { useToast } from '@/hooks/use-toast';

interface Connection {
  id: number;
  provider: string;
  name: string;
  api_key: string;
  base_url: string;
  status: string;
}

interface Combo {
  id: number;
  name: string;
  strategy: string;
  models_json: string;
}

interface Alias {
  id: number;
  alias_name: string;
  target_model: string;
  context_size: number;
}

interface UserKey {
  id: number;
  subdomain: string;
  name: string;
  key_value: string;
  created_at: string;
}

const PROVIDERS = [
  { value: 'openai', label: 'OpenAI' },
  { value: 'anthropic', label: 'Anthropic' },
  { value: 'groq', label: 'Groq (Free)' },
  { value: 'together', label: 'Together AI' },
  { value: 'openrouter', label: 'OpenRouter' },
  { value: 'custom', label: 'Custom / Ollama' },
];

const STRATEGIES = [
  { value: 'smart_context', label: 'Smart Context — route by context size (small -> small model, large -> large model)' },
  { value: 'fallback', label: 'Fallback — try in order, next if fail' },
  { value: 'round_robin', label: 'Round Robin — rotate each request' },
  { value: 'round_robin_sticky', label: 'Round Robin Sticky — rotate per session' },
];

const CONTEXT_PRESETS = [
  { value: 0, label: 'Auto / Default' },
  { value: 4096, label: '4k' },
  { value: 8192, label: '8k' },
  { value: 16384, label: '16k' },
  { value: 32768, label: '32k' },
  { value: 65536, label: '64k' },
  { value: 131072, label: '128k' },
  { value: 200000, label: '200k+' },
  { value: -1, label: 'Custom...' },
];

function formatContextSize(size: number): string {
  if (!size || size <= 0) return 'Auto';
  if (size >= 1000000) return `${(size / 1000000).toFixed(1)}M`;
  if (size >= 1000) return `${Math.round(size / 1000)}k`;
  return `${size}`;
}

const MODEL_SUGGESTIONS: Record<string, string[]> = {
  openai: ['openai/gpt-4o', 'openai/gpt-4o-mini', 'openai/o1-mini'],
  anthropic: ['anthropic/claude-3-5-sonnet-20241022', 'anthropic/claude-3-haiku-20240307'],
  groq: ['groq/llama-3.3-70b-versatile', 'groq/llama-3.1-8b-instant', 'groq/gemma2-9b-it'],
  together: ['together/meta-llama/Llama-3-70b-chat-hf'],
  openrouter: ['openrouter/anthropic/claude-3.5-sonnet', 'openrouter/openai/gpt-4o'],
};

interface RouterSectionProps {
  title: string;
  count?: number;
  expanded: boolean;
  onToggle: () => void;
  children: React.ReactNode;
}

function RouterSection({ title, count, expanded, onToggle, children }: RouterSectionProps) {
  return (
    <div className="bg-card rounded-none border border-border overflow-hidden">
      <button
        type="button"
        onClick={onToggle}
        className="w-full flex items-center justify-between px-4 py-3 bg-muted/20 hover:bg-muted/40 transition-colors"
      >
        <div className="flex items-center gap-2 font-semibold text-sm">
          {expanded ? <ChevronDown className="w-4 h-4 text-muted-foreground" /> : <ChevronRight className="w-4 h-4 text-muted-foreground" />}
          <span>{title}</span>
          {count !== undefined && (
            <span className="text-[11px] bg-background border border-border px-1.5 py-0.2 font-mono font-normal">
              {count}
            </span>
          )}
        </div>
      </button>
      {expanded && <div className="p-4 border-t border-border space-y-3">{children}</div>}
    </div>
  );
}

export function AIRouterUI({ subdomain }: { subdomain: string }) {
  const { toast } = useToast();
  const [loading, setLoading] = useState(true);

  const [connections, setConnections] = useState<Connection[]>([]);
  const [combos, setCombos] = useState<Combo[]>([]);
  const [aliases, setAliases] = useState<Alias[]>([]);
  const [userKeys, setUserKeys] = useState<UserKey[]>([]);
  const [newKeyName, setNewKeyName] = useState('');
  const [creatingKey, setCreatingKey] = useState(false);
  const [copiedKey, setCopiedKey] = useState('');
  const [vaultSecrets, setVaultSecrets] = useState<{ key: string; value: string }[]>([]);

  const [copiedUrl, setCopiedUrl] = useState('');
  const [showKeys, setShowKeys] = useState(false);
  const [expanded, setExpanded] = useState<Record<string, boolean>>({ connections: true, combos: false, aliases: false });

  // New connection form
  const [newConn, setNewConn] = useState({ provider: 'openai', name: '', api_key: '', base_url: '' });
  const [savingConn, setSavingConn] = useState(false);

  // New combo form
  const [newCombo, setNewCombo] = useState({ name: '', strategy: 'fallback', models: [''] });
  const [savingCombo, setSavingCombo] = useState(false);

  // New alias form
  const [newAlias, setNewAlias] = useState({ alias_name: '', target_model: '', context_size: 0 });
  const [isCustomCtx, setIsCustomCtx] = useState(false);
  const [savingAlias, setSavingAlias] = useState(false);



  const fetchAll = useCallback(async () => {
    try {
      const res = await fetch('/api/airouter/config', {
        headers: { 'X-Subdomain': subdomain }
      });
      if (!res.ok) throw new Error(await res.text());
      const data = await res.json();
      setConnections(data.connections || []);
      setCombos(data.combos || []);
      setAliases(data.aliases || []);
      setUserKeys(data.user_keys || []);

      // Fetch vault secrets for selection
      try {
        const vRes = await fetch('/api/vault/latest-kv', { headers: { 'X-Subdomain': subdomain } });
        if (vRes.ok) {
          const vData = await vRes.json();
          setVaultSecrets(vData || []);
        }
      } catch {}
    } catch (e: any) {
      toast({ title: 'Error', description: e.message, variant: 'destructive' });
    } finally {
      setLoading(false);
    }
  }, [subdomain]);

  useEffect(() => { fetchAll(); }, [fetchAll]);

  const copyUrl = (url: string) => {
    navigator.clipboard.writeText(url);
    setCopiedUrl(url);
    setTimeout(() => setCopiedUrl(''), 2000);
  };

  const toggle = (key: string) => setExpanded(e => ({ ...e, [key]: !e[key] }));

  const addConnection = async () => {
    if (!newConn.name || !newConn.api_key) {
      toast({ title: 'Name and API Key required', variant: 'destructive' });
      return;
    }
    setSavingConn(true);
    try {
      const res = await fetch('/api/airouter/connections', {
        method: 'POST',
        headers: { 'Content-Type': 'application/json', 'X-Subdomain': subdomain },
        body: JSON.stringify(newConn)
      });
      if (!res.ok) throw new Error(await res.text());
      setNewConn({ provider: 'openai', name: '', api_key: '', base_url: '' });
      await fetchAll();
      toast({ title: 'Connection added' });
    } catch (e: any) {
      toast({ title: 'Error', description: e.message, variant: 'destructive' });
    } finally {
      setSavingConn(false);
    }
  };

  const deleteConnection = async (id: number) => {
    await fetch(`/api/airouter/connections/${id}`, {
      method: 'DELETE', headers: { 'X-Subdomain': subdomain }
    });
    await fetchAll();
  };

  const saveCombo = async () => {
    const models = newCombo.models.filter(m => m.trim());
    if (!newCombo.name || models.length === 0) {
      toast({ title: 'Name and at least one model required', variant: 'destructive' });
      return;
    }
    setSavingCombo(true);
    try {
      const res = await fetch('/api/airouter/combos', {
        method: 'POST',
        headers: { 'Content-Type': 'application/json', 'X-Subdomain': subdomain },
        body: JSON.stringify({ ...newCombo, models_json: JSON.stringify(models) })
      });
      if (!res.ok) throw new Error(await res.text());
      setNewCombo({ name: '', strategy: 'fallback', models: [''] });
      await fetchAll();
      toast({ title: 'Combo saved' });
    } catch (e: any) {
      toast({ title: 'Error', description: e.message, variant: 'destructive' });
    } finally {
      setSavingCombo(false);
    }
  };

  const deleteCombo = async (id: number) => {
    await fetch(`/api/airouter/combos/${id}`, {
      method: 'DELETE', headers: { 'X-Subdomain': subdomain }
    });
    await fetchAll();
  };

  const saveAlias = async () => {
    if (!newAlias.alias_name || !newAlias.target_model) {
      toast({ title: 'Alias name and target required', variant: 'destructive' });
      return;
    }
    setSavingAlias(true);
    try {
      const res = await fetch('/api/airouter/aliases', {
        method: 'POST',
        headers: { 'Content-Type': 'application/json', 'X-Subdomain': subdomain },
        body: JSON.stringify(newAlias)
      });
      if (!res.ok) throw new Error(await res.text());
      setNewAlias({ alias_name: '', target_model: '', context_size: 0 });
      setIsCustomCtx(false);
      await fetchAll();
      toast({ title: 'Alias saved' });
    } catch (e: any) {
      toast({ title: 'Error', description: e.message, variant: 'destructive' });
    } finally {
      setSavingAlias(false);
    }
  };

  const deleteAlias = async (id: number) => {
    await fetch(`/api/airouter/aliases/${id}`, {
      method: 'DELETE', headers: { 'X-Subdomain': subdomain }
    });
    await fetchAll();
  };

  const copyKey = (key: string) => {
    navigator.clipboard.writeText(key);
    setCopiedKey(key);
    setTimeout(() => setCopiedKey(''), 2000);
  };

  const createKey = async () => {
    setCreatingKey(true);
    try {
      const res = await fetch('/api/airouter/keys', {
        method: 'POST',
        headers: { 'Content-Type': 'application/json', 'X-Subdomain': subdomain },
        body: JSON.stringify({ name: newKeyName.trim() || 'Default Client' })
      });
      if (!res.ok) throw new Error(await res.text());
      setNewKeyName('');
      await fetchAll();
      toast({ title: 'API Key created' });
    } catch (e: any) {
      toast({ title: 'Error', description: e.message, variant: 'destructive' });
    } finally {
      setCreatingKey(false);
    }
  };

  const deleteKey = async (id: number) => {
    try {
      const res = await fetch(`/api/airouter/keys/${id}`, {
        method: 'DELETE',
        headers: { 'X-Subdomain': subdomain }
      });
      if (!res.ok) throw new Error(await res.text());
      await fetchAll();
      toast({ title: 'API Key revoked' });
    } catch (e: any) {
      toast({ title: 'Error', description: e.message, variant: 'destructive' });
    }
  };



  if (loading) return (
    <div className="flex justify-center items-center h-64">
      <Loader2 className="w-8 h-8 animate-spin text-primary" />
    </div>
  );

  const openaiUrl = `https://${subdomain}.router.zcdns.id/v1/chat/completions`;
  const anthropicUrl = `https://${subdomain}.router.zcdns.id/v1/messages`;

  return (
    <div className="space-y-4">
      {/* Header */}
      <div className="flex items-center justify-between">
        <div>
          <h2 className="text-xl font-bold tracking-tight flex items-center gap-2">
            <Network className="w-5 h-5 text-primary" /> AI Router
          </h2>
          <p className="text-xs text-muted-foreground mt-0.5">
            Personal AI proxy.
          </p>
        </div>
      </div>

      {/* Endpoint URLs */}
      <div className="bg-card border border-border rounded-none p-4 space-y-3">
        <div>
          <h3 className="font-semibold text-sm mb-0.5">Proxy Endpoints</h3>
          <p className="text-xs text-muted-foreground">
            Endpoints for clients.
          </p>
        </div>

        <div className="space-y-2">
          {[
            { label: 'OpenAI-compatible (/v1/chat/completions)', url: openaiUrl },
            { label: 'Anthropic-compatible (/v1/messages)', url: anthropicUrl },
          ].map(({ label, url }) => (
            <div key={url}>
              <span className="text-[11px] font-medium text-muted-foreground">{label}</span>
              <div className="flex mt-1">
                <input readOnly value={url} className="flex-1 bg-muted px-3 py-1.5 text-xs font-mono rounded-none border border-border" />
                <button onClick={() => copyUrl(url)} className="px-3 border border-l-0 border-border rounded-none hover:bg-muted flex items-center">
                  {copiedUrl === url ? <Check className="w-3.5 h-3.5 text-green-500" /> : <Copy className="w-3.5 h-3.5" />}
                </button>
              </div>
            </div>
          ))}
        </div>

        <div className="flex items-center gap-2 text-xs text-primary/80 bg-primary/5 border border-primary/10 rounded-none px-2.5 py-1.5">
          <span>✨</span>
          <span><strong>Auto Format:</strong> Format auto-detected. Output matches input.</span>
        </div>

        {/* Client API Keys Management */}
        <div className="pt-3 border-t border-border space-y-2.5">
          <div className="flex items-center justify-between">
            <div>
              <h4 className="text-xs font-semibold flex items-center gap-1.5">
                <Key className="w-3.5 h-3.5 text-primary" /> Client Keys
              </h4>
              <p className="text-[11px] text-muted-foreground mt-0.5">
                {userKeys.length > 0
                  ? 'Pass key via Bearer or x-api-key.'
                  : 'Open access. Add key to secure.'}
              </p>
            </div>
            {userKeys.length > 0 && (
              <span className="text-[11px] bg-primary/10 text-primary font-mono px-1.5 py-0.5 rounded-none">
                {userKeys.length} {userKeys.length === 1 ? 'key' : 'keys'}
              </span>
            )}
          </div>

          {/* Create key form */}
          <div className="flex gap-2 items-center flex-wrap">
            <input
              value={newKeyName}
              onChange={e => setNewKeyName(e.target.value)}
              placeholder="Client label (e.g. Cursor)"
              className="flex-1 min-w-[200px] max-w-sm border border-border rounded-none px-2.5 py-1.5 text-xs bg-background"
            />
            <button
              onClick={createKey}
              disabled={creatingKey}
              className="flex items-center gap-1.5 px-3 py-1.5 bg-primary text-primary-foreground rounded-none text-xs font-medium hover:bg-primary/90 disabled:opacity-50"
            >
              {creatingKey ? <Loader2 className="w-3 h-3 animate-spin" /> : <Plus className="w-3 h-3" />}
              Add Key
            </button>
          </div>

          {/* List of keys */}
          {userKeys.length > 0 && (
            <div className="space-y-1.5 mt-2">
              {userKeys.map(k => (
                <div key={k.id} className="flex items-center justify-between gap-3 p-2.5 bg-muted/40 rounded-none border border-border text-xs flex-wrap">
                  <div className="flex items-center gap-2">
                    <ShieldCheck className="w-3.5 h-3.5 text-green-500 shrink-0" />
                    <span className="font-medium text-foreground">{k.name}</span>
                    <span className="text-[10px] text-muted-foreground">• {new Date(k.created_at).toLocaleDateString()}</span>
                  </div>
                  <div className="flex items-center gap-2">
                    <code className="font-mono text-xs bg-background border border-border px-2 py-0.5 rounded-none select-all">
                      {k.key_value}
                    </code>
                    <button onClick={() => copyKey(k.key_value)} className="p-1 hover:text-primary transition-colors" title="Copy key">
                      {copiedKey === k.key_value ? <Check className="w-3.5 h-3.5 text-green-500" /> : <Copy className="w-3.5 h-3.5" />}
                    </button>
                    <button onClick={() => deleteKey(k.id)} className="p-1 text-destructive hover:text-destructive/80 transition-colors" title="Revoke key">
                      <Trash2 className="w-3.5 h-3.5" />
                    </button>
                  </div>
                </div>
              ))}
            </div>
          )}
        </div>
      </div>

      {/* Connections */}
      <RouterSection title="Provider Connections" count={connections.length} expanded={!!expanded.connections} onToggle={() => toggle('connections')}>
        <div className="flex justify-between items-center mb-2">
          <p className="text-xs text-muted-foreground">Multi-key provider failover.</p>
          <button onClick={() => setShowKeys(!showKeys)} className="text-xs flex items-center gap-1 text-muted-foreground">
            {showKeys ? <><EyeOff className="w-3 h-3" /> Hide</> : <><Eye className="w-3 h-3" /> Show Keys</>}
          </button>
        </div>

        {/* Existing connections */}
        {connections.length > 0 && (
          <div className="mb-3 space-y-1.5">
            {connections.map(c => (
              <div key={c.id} className="flex items-center gap-2 p-2.5 bg-muted/40 rounded-none border border-border text-xs">
                <span className="font-mono text-[11px] bg-primary/10 text-primary px-1.5 py-0.5 rounded-none">{c.provider}</span>
                <span className="font-medium flex-1">{c.name}</span>
                <span className="font-mono text-muted-foreground text-xs">{showKeys ? c.api_key : c.api_key}</span>
                {c.status === 'active'
                  ? <CheckCircle2 className="w-3.5 h-3.5 text-green-500 shrink-0" />
                  : <span title="Rate limited"><AlertCircle className="w-3.5 h-3.5 text-yellow-500 shrink-0" /></span>}
                <button onClick={() => deleteConnection(c.id)} className="text-destructive hover:text-destructive/80 shrink-0">
                  <Trash2 className="w-3.5 h-3.5" />
                </button>
              </div>
            ))}
          </div>
        )}

        {/* Add new connection form */}
        <div className="border border-border rounded-none p-3 space-y-2.5 bg-card">
          <p className="text-[11px] font-semibold text-muted-foreground uppercase tracking-wide">Add Connection</p>
          <div className="grid grid-cols-1 sm:grid-cols-2 gap-2.5">
            <div>
              <label className="text-xs mb-1 block">Provider</label>
              <select value={newConn.provider} onChange={e => setNewConn(c => ({ ...c, provider: e.target.value }))}
                className="w-full border border-border rounded-none px-2 py-1.5 text-xs bg-background">
                {PROVIDERS.map(p => <option key={p.value} value={p.value}>{p.label}</option>)}
              </select>
            </div>
            <div>
              <label className="text-xs mb-1 block">Label</label>
              <input value={newConn.name} onChange={e => setNewConn(c => ({ ...c, name: e.target.value }))}
                placeholder="My OpenAI key" className="w-full border border-border rounded-none px-2 py-1.5 text-xs bg-background" />
            </div>
            <div>
              <div className="flex justify-between items-center mb-1">
                <label className="text-xs">API Key</label>
                {vaultSecrets.length > 0 ? (
                  <select
                    className="text-[11px] text-primary bg-primary/10 hover:bg-primary/20 border border-primary/20 rounded-none px-1.5 py-0.5 cursor-pointer font-medium"
                    onChange={e => {
                      const found = vaultSecrets.find(s => s.key === e.target.value);
                      if (found) {
                        setNewConn(c => ({
                          ...c,
                          api_key: found.value,
                          name: c.name || `Vault: ${found.key}`
                        }));
                        toast({ title: `Key '${found.key}' selected from Vault` });
                      }
                      e.target.value = "";
                    }}
                    defaultValue=""
                  >
                    <option value="" disabled>⚡ Select from Vault ({vaultSecrets.length} keys)...</option>
                    {vaultSecrets.map(s => (
                      <option key={s.key} value={s.key}>{s.key}</option>
                    ))}
                  </select>
                ) : (
                  <span className="text-[10px] text-muted-foreground">(Vault empty)</span>
                )}
              </div>
              <input type="password" value={newConn.api_key} onChange={e => setNewConn(c => ({ ...c, api_key: e.target.value }))}
                placeholder="sk-... or choose from Vault" className="w-full border border-border rounded-none px-2 py-1.5 text-xs bg-background font-mono" />
            </div>
            {newConn.provider === 'custom' && (
              <div>
                <div className="flex justify-between items-center mb-1">
                  <label className="text-xs">Base URL</label>
                  {vaultSecrets.length > 0 ? (
                    <select
                      className="text-[11px] text-primary bg-primary/10 hover:bg-primary/20 border border-primary/20 rounded-none px-1.5 py-0.5 cursor-pointer font-medium"
                      onChange={e => {
                        const found = vaultSecrets.find(s => s.key === e.target.value);
                        if (found) {
                          setNewConn(c => ({
                            ...c,
                            base_url: found.value,
                            name: c.name || `Vault: ${found.key}`
                          }));
                          toast({ title: `Base URL '${found.key}' selected from Vault` });
                        }
                        e.target.value = "";
                      }}
                      defaultValue=""
                    >
                      <option value="" disabled>⚡ Select from Vault ({vaultSecrets.length} keys)...</option>
                      {vaultSecrets.map(s => (
                        <option key={s.key} value={s.key}>{s.key}</option>
                      ))}
                    </select>
                  ) : (
                    <span className="text-[10px] text-muted-foreground">(Vault empty)</span>
                  )}
                </div>
                <input value={newConn.base_url} onChange={e => setNewConn(c => ({ ...c, base_url: e.target.value }))}
                  placeholder="https://localhost:11434 or choose from Vault" className="w-full border border-border rounded-none px-2 py-1.5 text-xs bg-background" />
              </div>
            )}
          </div>
          <button onClick={addConnection} disabled={savingConn}
            className="flex items-center gap-1.5 px-3 py-1.5 bg-primary text-primary-foreground rounded-none text-xs font-medium">
            {savingConn ? <Loader2 className="w-3 h-3 animate-spin" /> : <Plus className="w-3 h-3" />} Add Connection
          </button>
        </div>
      </RouterSection>

      {/* Combos */}
      <RouterSection title="Model Combos" count={combos.length} expanded={!!expanded.combos} onToggle={() => toggle('combos')}>
        <p className="text-xs text-muted-foreground mb-2">
          Group models by strategy.
        </p>

        {combos.length > 0 && (
          <div className="mb-3 space-y-1.5">
            {combos.map(c => {
              const models: string[] = JSON.parse(c.models_json || '[]');
              return (
                <div key={c.id} className="p-2.5 bg-muted/40 rounded-none border border-border text-xs">
                  <div className="flex items-center gap-2">
                    <code className="font-mono font-bold text-primary">{c.name}</code>
                    <span className="text-[11px] bg-secondary px-1.5 py-0.5 rounded-none">{c.strategy}</span>
                    <button onClick={() => deleteCombo(c.id)} className="ml-auto text-destructive">
                      <Trash2 className="w-3.5 h-3.5" />
                    </button>
                  </div>
                  <div className="mt-1 flex flex-wrap gap-1">
                    {models.map(m => <span key={m} className="text-[11px] bg-muted border border-border px-1.5 py-0.5 rounded-none font-mono">{m}</span>)}
                  </div>
                </div>
              );
            })}
          </div>
        )}

        <div className="border border-border rounded-none p-3 space-y-2.5 bg-card">
          <p className="text-[11px] font-semibold text-muted-foreground uppercase tracking-wide">Add Combo</p>
          <div className="grid grid-cols-1 sm:grid-cols-2 gap-2.5">
            <div>
              <label className="text-xs mb-1 block">Combo Name</label>
              <input value={newCombo.name} onChange={e => setNewCombo(c => ({ ...c, name: e.target.value }))}
                placeholder="best-coding" className="w-full border border-border rounded-none px-2 py-1.5 text-xs bg-background font-mono" />
            </div>
            <div>
              <label className="text-xs mb-1 block">Strategy</label>
              <select value={newCombo.strategy} onChange={e => setNewCombo(c => ({ ...c, strategy: e.target.value }))}
                className="w-full border border-border rounded-none px-2 py-1.5 text-xs bg-background">
                {STRATEGIES.map(s => <option key={s.value} value={s.value}>{s.label}</option>)}
              </select>
            </div>
          </div>
          <div className="space-y-1.5">
            <label className="text-xs block">Models</label>
            {newCombo.models.map((m, i) => (
              <div key={i} className="flex gap-2">
                <input value={m} onChange={e => {
                  const models = [...newCombo.models];
                  models[i] = e.target.value;
                  setNewCombo(c => ({ ...c, models }));
                }}
                  list={`model-suggestions-${i}`}
                  placeholder="openai/gpt-4o"
                  className="flex-1 border border-border rounded-none px-2 py-1.5 text-xs bg-background font-mono" />
                <datalist id={`model-suggestions-${i}`}>
                  {Object.values(MODEL_SUGGESTIONS).flat().map(s => <option key={s} value={s} />)}
                </datalist>
                {newCombo.models.length > 1 && (
                  <button onClick={() => setNewCombo(c => ({ ...c, models: c.models.filter((_, j) => j !== i) }))}
                    className="text-destructive px-2"><Trash2 className="w-3.5 h-3.5" /></button>
                )}
              </div>
            ))}
            <button onClick={() => setNewCombo(c => ({ ...c, models: [...c.models, ''] }))}
              className="text-xs text-primary flex items-center gap-1 mt-1">
              <Plus className="w-3 h-3" /> Add model
            </button>
          </div>
          <button onClick={saveCombo} disabled={savingCombo}
            className="flex items-center gap-1.5 px-3 py-1.5 bg-primary text-primary-foreground rounded-none text-xs font-medium">
            {savingCombo ? <Loader2 className="w-3 h-3 animate-spin" /> : <Save className="w-3 h-3" />} Save Combo
          </button>
        </div>
      </RouterSection>

      {/* Aliases */}
      <RouterSection title="Model Aliases" count={aliases.length} expanded={!!expanded.aliases} onToggle={() => toggle('aliases')}>
        <p className="text-xs text-muted-foreground mb-2">
          Map aliases to models.
        </p>

        {aliases.length > 0 && (
          <div className="mb-3 space-y-1.5">
            {aliases.map(a => (
              <div key={a.id} className="flex items-center gap-3 p-2.5 bg-muted/40 rounded-none border border-border text-xs">
                <code className="font-mono font-bold text-primary">{a.alias_name}</code>
                <span className="text-muted-foreground">→</span>
                <code className="font-mono text-xs flex-1">{a.target_model}</code>
                <span className="text-[11px] bg-primary/10 text-primary px-1.5 py-0.5 rounded-none font-mono">
                  {formatContextSize(a.context_size)} ctx
                </span>
                <button onClick={() => deleteAlias(a.id)} className="text-destructive">
                  <Trash2 className="w-3.5 h-3.5" />
                </button>
              </div>
            ))}
          </div>
        )}

        <div className="border border-border rounded-none p-3 space-y-2.5 bg-card">
          <p className="text-[11px] font-semibold text-muted-foreground uppercase tracking-wide">Add Alias</p>
          <div className="flex gap-2 flex-wrap items-center">
            <input value={newAlias.alias_name} onChange={e => setNewAlias(a => ({ ...a, alias_name: e.target.value }))}
              placeholder="claude" className="border border-border rounded-none px-2 py-1.5 text-xs bg-background font-mono w-28" />
            <span className="self-center text-muted-foreground">→</span>
            <input value={newAlias.target_model} onChange={e => setNewAlias(a => ({ ...a, target_model: e.target.value }))}
              list="alias-target-list"
              placeholder="anthropic/claude-3-5-sonnet or combo"
              className="flex-1 min-w-[180px] border border-border rounded-none px-2 py-1.5 text-xs bg-background font-mono" />
            <datalist id="alias-target-list">
              {Object.values(MODEL_SUGGESTIONS).flat().map(s => <option key={s} value={s} />)}
              {combos.map(c => <option key={c.name} value={c.name} />)}
            </datalist>

            <div className="flex items-center gap-1.5">
              <label className="text-xs text-muted-foreground whitespace-nowrap">Context:</label>
              <select
                value={isCustomCtx ? -1 : (newAlias.context_size || 0)}
                onChange={e => {
                  const val = parseInt(e.target.value, 10);
                  if (val === -1) {
                    setIsCustomCtx(true);
                  } else {
                    setIsCustomCtx(false);
                    setNewAlias(a => ({ ...a, context_size: val }));
                  }
                }}
                className="border border-border rounded-none px-2 py-1.5 text-xs bg-background font-mono"
              >
                {CONTEXT_PRESETS.map(p => (
                  <option key={p.value} value={p.value}>{p.label}</option>
                ))}
              </select>
              {isCustomCtx && (
                <input
                  type="number"
                  placeholder="Tokens"
                  value={newAlias.context_size || ''}
                  onChange={e => setNewAlias(a => ({ ...a, context_size: parseInt(e.target.value, 10) || 0 }))}
                  className="border border-border rounded-none px-2 py-1.5 text-xs bg-background font-mono w-24"
                />
              )}
            </div>

            <button onClick={saveAlias} disabled={savingAlias}
              className="flex items-center gap-1.5 px-3 py-1.5 bg-primary text-primary-foreground rounded-none text-xs font-medium">
              {savingAlias ? <Loader2 className="w-3 h-3 animate-spin" /> : <Plus className="w-3 h-3" />} Add
            </button>
          </div>
        </div>
      </RouterSection>

      {/* OpenWebUI */}
      <div className="bg-card border border-border rounded-none p-4 space-y-3">
        <div className="flex items-center justify-between flex-wrap gap-2">
          <div>
            <h3 className="font-semibold text-sm flex items-center gap-1.5">
              <MessageSquare className="w-4 h-4 text-primary" /> OpenWebUI
            </h3>
            <p className="text-xs text-muted-foreground mt-0.5">
              Visual chat interface.
            </p>
          </div>
          <a
            href={userKeys.length > 0
              ? `https://${subdomain}.router.zcdns.id/?key=${encodeURIComponent(userKeys[0].key_value)}`
              : `https://${subdomain}.router.zcdns.id/`}
            target="_blank"
            rel="noopener noreferrer"
            className="flex items-center gap-1.5 px-3 py-1.5 bg-primary text-primary-foreground text-xs font-medium rounded-none hover:bg-primary/90 transition-colors"
          >
            <span>Open WebUI</span>
            <ExternalLink className="w-3.5 h-3.5" />
          </a>
        </div>
        <p className="text-xs text-muted-foreground">
          Chat directly with your configured models, combos, and aliases in an interactive OpenWebUI workspace at{' '}
          <code className="font-mono text-primary bg-muted px-1 py-0.5 border border-border">https://{subdomain}.router.zcdns.id</code>.
        </p>
      </div>
    </div>
  );
}
