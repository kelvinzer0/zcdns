import React, { useState, useEffect, useCallback } from 'react';
import {
  Network, Plus, Trash2, Save, Copy, Check, Eye, EyeOff,
  Loader2, ChevronDown, ChevronRight, AlertCircle, CheckCircle2,
  Key, ShieldCheck, ExternalLink, MessageSquare, RefreshCw, Zap, X
} from 'lucide-react';
import { useToast } from '@/hooks/use-toast';

interface Connection {
  id: number;
  provider: string;
  api_type: string; // 'openai' | 'anthropic'
  name: string;
  api_key: string;
  base_url: string;
  models_json?: string;
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

interface ProviderDef {
  value: string;
  label: string;
  defaultType: 'openai' | 'anthropic';
  defaultBaseUrl: string;
}

const PROVIDERS: ProviderDef[] = [
  { value: 'openai', label: 'OpenAI (Official)', defaultType: 'openai', defaultBaseUrl: 'https://api.openai.com' },
  { value: 'anthropic', label: 'Anthropic (Official)', defaultType: 'anthropic', defaultBaseUrl: 'https://api.anthropic.com' },
  { value: 'deepseek', label: 'DeepSeek', defaultType: 'openai', defaultBaseUrl: 'https://api.deepseek.com' },
  { value: 'groq', label: 'Groq (Free & Fast)', defaultType: 'openai', defaultBaseUrl: 'https://api.groq.com/openai' },
  { value: 'together', label: 'Together AI', defaultType: 'openai', defaultBaseUrl: 'https://api.together.xyz' },
  { value: 'openrouter', label: 'OpenRouter', defaultType: 'openai', defaultBaseUrl: 'https://openrouter.ai/api' },
  { value: 'custom', label: 'Custom / Local / Ollama Proxy', defaultType: 'openai', defaultBaseUrl: 'http://localhost:11434' },
];

const API_TYPES = [
  { value: 'openai', label: 'OpenAI-compatible (/v1/chat/completions)' },
  { value: 'anthropic', label: 'Anthropic-compatible (/v1/messages)' },
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

const BUILTIN_MODEL_SUGGESTIONS: Record<string, string[]> = {
  openai: ['gpt-4o', 'gpt-4o-mini', 'o1', 'o3-mini'],
  anthropic: ['claude-3-5-sonnet-20241022', 'claude-3-5-haiku-20241022', 'claude-3-opus-20240229'],
  deepseek: ['deepseek-chat', 'deepseek-reasoner'],
  groq: ['groq/llama-3.3-70b-versatile', 'groq/llama-3.1-8b-instant', 'groq/gemma2-9b-it'],
  together: ['meta-llama/Meta-Llama-3.1-70B-Instruct-Turbo', 'meta-llama/Meta-Llama-3.1-8B-Instruct-Turbo'],
  openrouter: ['openrouter/auto', 'anthropic/claude-3.5-sonnet', 'openai/gpt-4o'],
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
  const [newConn, setNewConn] = useState({
    provider: 'openai',
    api_type: 'openai',
    name: '',
    api_key: '',
    base_url: 'https://api.openai.com',
    models: [] as string[]
  });
  const [manualModelInput, setManualModelInput] = useState('');
  const [savingConn, setSavingConn] = useState(false);
  const [testingConn, setTestingConn] = useState(false);
  const [testResult, setTestResult] = useState<{
    success: boolean;
    latency_ms?: number;
    message?: string;
    error?: string;
    models?: string[];
  } | null>(null);

  // Connection-specific active tests and model drawer
  const [testingConnId, setTestingConnId] = useState<number | null>(null);
  const [editingModelsConnId, setEditingModelsConnId] = useState<number | null>(null);
  const [editingModelsList, setEditingModelsList] = useState<string[]>([]);
  const [editingModelInput, setEditingModelInput] = useState('');
  const [savingConnModels, setSavingConnModels] = useState(false);

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

  // Aggregate all discovered/configured models across all connections
  const allDiscoveredModels = Array.from(new Set([
    ...connections.flatMap(c => {
      try {
        return JSON.parse(c.models_json || '[]');
      } catch {
        return [];
      }
    }),
    ...newConn.models,
    ...Object.values(BUILTIN_MODEL_SUGGESTIONS).flat()
  ])).filter(Boolean);

  const copyUrl = (url: string) => {
    navigator.clipboard.writeText(url);
    setCopiedUrl(url);
    setTimeout(() => setCopiedUrl(''), 2000);
  };

  const toggle = (key: string) => setExpanded(e => ({ ...e, [key]: !e[key] }));

  // Handle provider selection change
  const handleProviderChange = (providerVal: string) => {
    const def = PROVIDERS.find(p => p.value === providerVal);
    if (!def) return;
    setNewConn(c => ({
      ...c,
      provider: def.value,
      api_type: def.defaultType,
      base_url: def.defaultBaseUrl,
      name: c.name || `${def.label} Connection`
    }));
    setTestResult(null);
  };

  // Test connection (for new form or existing connection)
  const testConnection = async (targetConn?: { id?: number; provider: string; api_type: string; api_key: string; base_url: string }) => {
    const isNew = !targetConn?.id;
    if (isNew) {
      if (!newConn.api_key) {
        toast({ title: 'API Key is required to test connection', variant: 'destructive' });
        return;
      }
      setTestingConn(true);
      setTestResult(null);
    } else {
      setTestingConnId(targetConn.id!);
    }

    try {
      const payload = isNew
        ? {
            provider: newConn.provider,
            api_type: newConn.api_type,
            api_key: newConn.api_key,
            base_url: newConn.base_url
          }
        : {
            id: targetConn.id,
            provider: targetConn.provider,
            api_type: targetConn.api_type,
            api_key: targetConn.api_key,
            base_url: targetConn.base_url
          };

      const res = await fetch('/api/airouter/connections/test', {
        method: 'POST',
        headers: { 'Content-Type': 'application/json', 'X-Subdomain': subdomain },
        body: JSON.stringify(payload)
      });
      const data = await res.json();

      if (isNew) {
        setTestResult(data);
        if (data.success && data.models?.length) {
          setNewConn(c => ({
            ...c,
            models: Array.from(new Set([...c.models, ...data.models]))
          }));
        }
      }

      if (data.success) {
        toast({
          title: 'Connection Succeeded!',
          description: `${data.message} (latency: ${data.latency_ms}ms, ${data.models?.length || 0} models found)`
        });
        if (!isNew) {
          await fetchAll();
        }
      } else {
        toast({
          title: 'Connection Failed',
          description: data.error || 'Check credentials or base URL',
          variant: 'destructive'
        });
      }
    } catch (e: any) {
      const msg = e.message || 'Network test error';
      if (isNew) {
        setTestResult({ success: false, error: msg });
      }
      toast({ title: 'Test Failed', description: msg, variant: 'destructive' });
    } finally {
      if (isNew) {
        setTestingConn(false);
      } else {
        setTestingConnId(null);
      }
    }
  };

  // Add a manual model to the new connection
  const addManualModelToNewConn = () => {
    const val = manualModelInput.trim();
    if (!val) return;
    if (!newConn.models.includes(val)) {
      setNewConn(c => ({ ...c, models: [...c.models, val] }));
    }
    setManualModelInput('');
  };

  const removeModelFromNewConn = (modelName: string) => {
    setNewConn(c => ({ ...c, models: c.models.filter(m => m !== modelName) }));
  };

  const addConnection = async () => {
    if (!newConn.name || !newConn.api_key) {
      toast({ title: 'Name and API Key are required', variant: 'destructive' });
      return;
    }
    setSavingConn(true);
    try {
      const res = await fetch('/api/airouter/connections', {
        method: 'POST',
        headers: { 'Content-Type': 'application/json', 'X-Subdomain': subdomain },
        body: JSON.stringify({
          provider: newConn.provider,
          api_type: newConn.api_type,
          name: newConn.name,
          api_key: newConn.api_key,
          base_url: newConn.base_url,
          models_json: JSON.stringify(newConn.models)
        })
      });
      if (!res.ok) throw new Error(await res.text());
      setNewConn({
        provider: 'openai',
        api_type: 'openai',
        name: '',
        api_key: '',
        base_url: 'https://api.openai.com',
        models: []
      });
      setTestResult(null);
      await fetchAll();
      toast({ title: 'Connection added successfully' });
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

  // Open model management for an existing connection
  const openModelEditor = (c: Connection) => {
    let parsed: string[] = [];
    try {
      parsed = JSON.parse(c.models_json || '[]');
    } catch {}
    setEditingModelsConnId(c.id);
    setEditingModelsList(parsed);
    setEditingModelInput('');
  };

  const saveConnectionModels = async (id: number) => {
    setSavingConnModels(true);
    try {
      const res = await fetch(`/api/airouter/connections/${id}/models`, {
        method: 'POST',
        headers: { 'Content-Type': 'application/json', 'X-Subdomain': subdomain },
        body: JSON.stringify({ models: editingModelsList })
      });
      if (!res.ok) throw new Error(await res.text());
      await fetchAll();
      setEditingModelsConnId(null);
      toast({ title: 'Connection models updated' });
    } catch (e: any) {
      toast({ title: 'Error', description: e.message, variant: 'destructive' });
    } finally {
      setSavingConnModels(false);
    }
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
            Personal AI gateway with multi-provider failover, smart routing, and model management.
          </p>
        </div>
      </div>

      {/* Endpoint URLs */}
      <div className="bg-card border border-border rounded-none p-4 space-y-3">
        <div>
          <h3 className="font-semibold text-sm mb-0.5">Proxy Endpoints</h3>
          <p className="text-xs text-muted-foreground">
            Connect Claude Code, Cursor, Cline, OpenWebUI, or any OpenAI / Anthropic client.
          </p>
        </div>

        <div className="space-y-2">
          {[
            { label: 'OpenAI-compatible (/v1/chat/completions)', url: openaiUrl },
            { label: 'Anthropic-compatible (/v1/messages)', url: anthropicUrl },
          ].map(ep => (
            <div key={ep.url} className="flex items-center justify-between gap-2 p-2 bg-muted/30 rounded-none border border-border text-xs">
              <span className="font-mono text-muted-foreground text-[11px] truncate flex-1">{ep.label}: <strong className="text-foreground">{ep.url}</strong></span>
              <button
                onClick={() => copyUrl(ep.url)}
                className="flex items-center gap-1 px-2 py-1 bg-background hover:bg-muted border border-border text-foreground transition-colors shrink-0 font-medium"
              >
                {copiedUrl === ep.url ? <Check className="w-3.5 h-3.5 text-green-500" /> : <Copy className="w-3.5 h-3.5" />}
                <span>{copiedUrl === ep.url ? 'Copied' : 'Copy'}</span>
              </button>
            </div>
          ))}
        </div>

        {/* Security / Access Keys */}
        <div className="pt-2 border-t border-border space-y-2">
          <div className="flex items-center justify-between">
            <div>
              <span className="text-xs font-semibold flex items-center gap-1.5">
                <Key className="w-3.5 h-3.5 text-primary" /> Client Access Keys
              </span>
              <p className="text-[11px] text-muted-foreground">
                {userKeys.length > 0
                  ? 'Protected mode active. Only clients presenting a valid key can use your proxy.'
                  : 'Open access. Add at least one key to secure your proxy.'}
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
              placeholder="Client label (e.g. Cursor, Claude Code)"
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

      {/* ── PROVIDER CONNECTIONS ────────────────────────────────────────────── */}
      <RouterSection title="Provider Connections" count={connections.length} expanded={!!expanded.connections} onToggle={() => toggle('connections')}>
        <div className="flex justify-between items-center mb-2">
          <p className="text-xs text-muted-foreground">
            Configure upstream AI providers with failover, custom protocols (OpenAI / Anthropic), and model lists.
          </p>
          <button onClick={() => setShowKeys(!showKeys)} className="text-xs flex items-center gap-1 text-muted-foreground hover:text-foreground">
            {showKeys ? <><EyeOff className="w-3 h-3" /> Hide</> : <><Eye className="w-3 h-3" /> Show Keys</>}
          </button>
        </div>

        {/* Existing connections */}
        {connections.length > 0 && (
          <div className="mb-3 space-y-2">
            {connections.map(c => {
              let connModels: string[] = [];
              try {
                connModels = JSON.parse(c.models_json || '[]');
              } catch {}

              const isEditingThis = editingModelsConnId === c.id;
              const isTestingThis = testingConnId === c.id;

              return (
                <div key={c.id} className="p-3 bg-muted/40 rounded-none border border-border text-xs space-y-2">
                  <div className="flex items-center gap-2 flex-wrap">
                    <span className="font-mono text-[11px] bg-primary/10 text-primary font-bold px-1.5 py-0.5 rounded-none uppercase">
                      {c.provider}
                    </span>
                    <span className="text-[10px] font-mono bg-secondary px-1.5 py-0.5 rounded-none uppercase border border-border">
                      {c.api_type || (c.provider === 'anthropic' ? 'anthropic' : 'openai')}
                    </span>
                    <span className="font-semibold text-foreground flex-1 min-w-[120px]">{c.name}</span>

                    <span className="font-mono text-muted-foreground text-xs">
                      {showKeys ? c.api_key : (c.api_key.includes('...') ? c.api_key : c.api_key.slice(0, 4) + '...' + c.api_key.slice(-4))}
                    </span>

                    {/* Status badge */}
                    {c.status === 'active' ? (
                      <span className="flex items-center gap-1 text-green-500 font-medium text-[11px]">
                        <CheckCircle2 className="w-3.5 h-3.5" /> Active
                      </span>
                    ) : (
                      <span className="flex items-center gap-1 text-yellow-500 font-medium text-[11px]" title="Rate limited">
                        <AlertCircle className="w-3.5 h-3.5" /> Rate Limited
                      </span>
                    )}

                    {/* Test Button for Existing */}
                    <button
                      type="button"
                      disabled={isTestingThis}
                      onClick={() => testConnection(c)}
                      className="flex items-center gap-1 px-2 py-1 bg-background hover:bg-muted border border-border text-[11px] font-medium transition-colors"
                      title="Test credentials and latency"
                    >
                      {isTestingThis ? <Loader2 className="w-3 h-3 animate-spin text-primary" /> : <Zap className="w-3 h-3 text-yellow-500" />}
                      <span>Test</span>
                    </button>

                    {/* Model editor toggle */}
                    <button
                      type="button"
                      onClick={() => isEditingThis ? setEditingModelsConnId(null) : openModelEditor(c)}
                      className="flex items-center gap-1 px-2 py-1 bg-background hover:bg-muted border border-border text-[11px] font-medium transition-colors"
                    >
                      <RefreshCw className="w-3 h-3 text-primary" />
                      <span>{connModels.length > 0 ? `${connModels.length} Models` : 'Models'}</span>
                    </button>

                    <button
                      onClick={() => deleteConnection(c.id)}
                      className="text-destructive hover:text-destructive/80 p-1 transition-colors"
                      title="Delete connection"
                    >
                      <Trash2 className="w-3.5 h-3.5" />
                    </button>
                  </div>

                  {c.base_url && (
                    <div className="text-[11px] text-muted-foreground font-mono truncate">
                      Base URL: <span className="text-foreground">{c.base_url}</span>
                    </div>
                  )}

                  {/* Model Management Sub-Drawer for Existing Connection */}
                  {isEditingThis ? (
                    <div className="p-2.5 bg-background border border-border mt-2 space-y-2">
                      <div className="flex items-center justify-between">
                        <span className="font-semibold text-xs text-foreground">Models for {c.name}</span>
                        <button
                          onClick={() => testConnection(c)}
                          disabled={isTestingThis}
                          className="text-[11px] text-primary hover:underline flex items-center gap-1 font-medium"
                        >
                          {isTestingThis ? <Loader2 className="w-3 h-3 animate-spin" /> : <RefreshCw className="w-3 h-3" />}
                          Auto-fetch from provider
                        </button>
                      </div>

                      {/* Chips */}
                      <div className="flex flex-wrap gap-1.5 max-h-36 overflow-y-auto">
                        {editingModelsList.length === 0 ? (
                          <p className="text-[11px] text-muted-foreground italic">
                            No models defined yet. Click auto-fetch or enter manual model names below.
                          </p>
                        ) : (
                          editingModelsList.map(m => (
                            <span key={m} className="inline-flex items-center gap-1 text-[11px] bg-muted border border-border px-2 py-0.5 rounded-none font-mono">
                              <span>{m}</span>
                              <button
                                type="button"
                                onClick={() => setEditingModelsList(l => l.filter(item => item !== m))}
                                className="hover:text-destructive ml-0.5"
                              >
                                <X className="w-3 h-3" />
                              </button>
                            </span>
                          ))
                        )}
                      </div>

                      {/* Manual input */}
                      <div className="flex gap-2 pt-1">
                        <input
                          value={editingModelInput}
                          onChange={e => setEditingModelInput(e.target.value)}
                          onKeyDown={e => {
                            if (e.key === 'Enter') {
                              e.preventDefault();
                              const val = editingModelInput.trim();
                              if (val && !editingModelsList.includes(val)) {
                                setEditingModelsList(l => [...l, val]);
                                setEditingModelInput('');
                              }
                            }
                          }}
                          placeholder="Type manual model name (e.g. gpt-4o, llama3.2)..."
                          className="flex-1 border border-border px-2 py-1 text-xs bg-background font-mono"
                        />
                        <button
                          type="button"
                          onClick={() => {
                            const val = editingModelInput.trim();
                            if (val && !editingModelsList.includes(val)) {
                              setEditingModelsList(l => [...l, val]);
                              setEditingModelInput('');
                            }
                          }}
                          className="px-2.5 py-1 bg-secondary text-secondary-foreground text-xs font-medium hover:bg-secondary/80 border border-border"
                        >
                          + Add
                        </button>
                        <button
                          type="button"
                          disabled={savingConnModels}
                          onClick={() => saveConnectionModels(c.id)}
                          className="px-3 py-1 bg-primary text-primary-foreground text-xs font-medium hover:bg-primary/90 flex items-center gap-1"
                        >
                          {savingConnModels ? <Loader2 className="w-3 h-3 animate-spin" /> : <Save className="w-3 h-3" />}
                          Save
                        </button>
                      </div>
                    </div>
                  ) : (
                    connModels.length > 0 && (
                      <div className="flex flex-wrap gap-1 pt-0.5">
                        {connModels.slice(0, 8).map(m => (
                          <span key={m} className="text-[10px] bg-muted/60 text-muted-foreground px-1.5 py-0.5 rounded-none font-mono border border-border/50">
                            {m}
                          </span>
                        ))}
                        {connModels.length > 8 && (
                          <span className="text-[10px] text-muted-foreground font-mono self-center">
                            +{connModels.length - 8} more
                          </span>
                        )}
                      </div>
                    )
                  )}
                </div>
              );
            })}
          </div>
        )}

        {/* ── ADD NEW CONNECTION FORM ───────────────────────────────────────── */}
        <div className="border border-border rounded-none p-3.5 space-y-3 bg-card">
          <div className="flex items-center justify-between">
            <p className="text-[11px] font-semibold text-muted-foreground uppercase tracking-wide">
              Add New Provider Connection
            </p>
          </div>

          <div className="grid grid-cols-1 sm:grid-cols-2 gap-3">
            {/* Provider Preset */}
            <div>
              <label className="text-xs mb-1 block font-medium">Provider</label>
              <select
                value={newConn.provider}
                onChange={e => handleProviderChange(e.target.value)}
                className="w-full border border-border rounded-none px-2.5 py-1.5 text-xs bg-background"
              >
                {PROVIDERS.map(p => (
                  <option key={p.value} value={p.value}>{p.label}</option>
                ))}
              </select>
            </div>

            {/* Connection Protocol / Type */}
            <div>
              <label className="text-xs mb-1 block font-medium">
                Connection Type / Protocol
              </label>
              <select
                value={newConn.api_type}
                onChange={e => setNewConn(c => ({ ...c, api_type: e.target.value }))}
                className="w-full border border-border rounded-none px-2.5 py-1.5 text-xs bg-background font-mono"
              >
                {API_TYPES.map(t => (
                  <option key={t.value} value={t.value}>{t.label}</option>
                ))}
              </select>
            </div>

            {/* Label */}
            <div>
              <label className="text-xs mb-1 block font-medium">Connection Label</label>
              <input
                value={newConn.name}
                onChange={e => setNewConn(c => ({ ...c, name: e.target.value }))}
                placeholder="e.g. My OpenAI Key, Home Ollama"
                className="w-full border border-border rounded-none px-2.5 py-1.5 text-xs bg-background"
              />
            </div>

            {/* Base URL */}
            <div>
              <div className="flex justify-between items-center mb-1">
                <label className="text-xs font-medium">
                  Base URL {newConn.provider !== 'custom' && <span className="text-[10px] text-muted-foreground font-normal">(Default provided, editable)</span>}
                </label>
                {vaultSecrets.length > 0 && (
                  <select
                    className="text-[11px] text-primary bg-primary/10 hover:bg-primary/20 border border-primary/20 px-1.5 py-0.5 cursor-pointer font-medium"
                    onChange={e => {
                      const found = vaultSecrets.find(s => s.key === e.target.value);
                      if (found) {
                        setNewConn(c => ({ ...c, base_url: found.value }));
                        toast({ title: `Base URL '${found.key}' selected from Vault` });
                      }
                      e.target.value = "";
                    }}
                    defaultValue=""
                  >
                    <option value="" disabled>⚡ Pick Base URL from Vault...</option>
                    {vaultSecrets.map(s => (
                      <option key={s.key} value={s.key}>{s.key}</option>
                    ))}
                  </select>
                )}
              </div>
              <input
                value={newConn.base_url}
                onChange={e => setNewConn(c => ({ ...c, base_url: e.target.value }))}
                placeholder="https://api.openai.com or http://localhost:11434"
                className="w-full border border-border rounded-none px-2.5 py-1.5 text-xs bg-background font-mono"
              />
            </div>

            {/* API Key with Vault Picker */}
            <div className="sm:col-span-2">
              <div className="flex justify-between items-center mb-1">
                <label className="text-xs font-medium">API Key / Secret Token</label>
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
                        toast({ title: `Key '${found.key}' loaded from Vault` });
                      }
                      e.target.value = "";
                    }}
                    defaultValue=""
                  >
                    <option value="" disabled>⚡ Load Key from Vault ({vaultSecrets.length} keys)...</option>
                    {vaultSecrets.map(s => (
                      <option key={s.key} value={s.key}>{s.key}</option>
                    ))}
                  </select>
                ) : (
                  <span className="text-[10px] text-muted-foreground">(Vault empty)</span>
                )}
              </div>
              <input
                type="password"
                value={newConn.api_key}
                onChange={e => setNewConn(c => ({ ...c, api_key: e.target.value }))}
                placeholder="sk-... or pick key from Vault"
                className="w-full border border-border rounded-none px-2.5 py-1.5 text-xs bg-background font-mono"
              />
            </div>
          </div>

          {/* Available / Manual Models Management in Add Form */}
          <div className="pt-2 border-t border-border space-y-2">
            <div className="flex items-center justify-between flex-wrap gap-2">
              <div>
                <label className="text-xs font-medium block">
                  Available Models ({newConn.models.length})
                </label>
                <p className="text-[11px] text-muted-foreground">
                  Fetch automatically using Test Connection / Fetch Models, or manually type models below.
                </p>
              </div>
              <button
                type="button"
                onClick={() => testConnection()}
                disabled={testingConn}
                className="flex items-center gap-1.5 px-2.5 py-1 bg-secondary text-secondary-foreground border border-border text-xs font-medium hover:bg-secondary/80 disabled:opacity-50"
              >
                {testingConn ? <Loader2 className="w-3.5 h-3.5 animate-spin text-primary" /> : <RefreshCw className="w-3.5 h-3.5 text-primary" />}
                <span>Fetch Models</span>
              </button>
            </div>

            {/* Chips of added models */}
            {newConn.models.length > 0 && (
              <div className="flex flex-wrap gap-1.5 max-h-32 overflow-y-auto p-2 bg-muted/20 border border-border">
                {newConn.models.map(m => (
                  <span key={m} className="inline-flex items-center gap-1 text-[11px] bg-background border border-border px-2 py-0.5 rounded-none font-mono">
                    <span>{m}</span>
                    <button
                      type="button"
                      onClick={() => removeModelFromNewConn(m)}
                      className="hover:text-destructive"
                    >
                      <X className="w-3 h-3" />
                    </button>
                  </span>
                ))}
              </div>
            )}

            {/* Manual model write input */}
            <div className="flex gap-2">
              <input
                value={manualModelInput}
                onChange={e => setManualModelInput(e.target.value)}
                onKeyDown={e => {
                  if (e.key === 'Enter') {
                    e.preventDefault();
                    addManualModelToNewConn();
                  }
                }}
                placeholder="Tulis model manual (e.g. deepseek-chat, gpt-4o, llama3.2:latest)..."
                className="flex-1 border border-border rounded-none px-2.5 py-1.5 text-xs bg-background font-mono"
              />
              <button
                type="button"
                onClick={addManualModelToNewConn}
                className="px-3 py-1.5 bg-secondary text-secondary-foreground rounded-none text-xs font-medium hover:bg-secondary/80 border border-border"
              >
                + Add Model
              </button>
            </div>
          </div>

          {/* Test Connection Result Feedback */}
          {testResult && (
            <div className={`p-2.5 border text-xs flex items-start gap-2 ${testResult.success ? 'bg-green-500/10 border-green-500/30 text-green-700 dark:text-green-400' : 'bg-destructive/10 border-destructive/30 text-destructive'}`}>
              {testResult.success ? (
                <CheckCircle2 className="w-4 h-4 shrink-0 mt-0.5" />
              ) : (
                <AlertCircle className="w-4 h-4 shrink-0 mt-0.5" />
              )}
              <div className="flex-1 space-y-1">
                <div className="font-semibold">
                  {testResult.success ? 'Connection Verified' : 'Connection Failed'}
                  {testResult.latency_ms !== undefined && ` (${testResult.latency_ms}ms latency)`}
                </div>
                <div className="text-[11px] leading-relaxed">
                  {testResult.success ? testResult.message : testResult.error}
                </div>
                {testResult.models && testResult.models.length > 0 && (
                  <div className="text-[11px] pt-1">
                    <span className="font-medium">Found {testResult.models.length} models: </span>
                    <span className="font-mono">{testResult.models.slice(0, 6).join(', ')}{testResult.models.length > 6 ? ` (+${testResult.models.length - 6} more)` : ''}</span>
                  </div>
                )}
              </div>
            </div>
          )}

          {/* Action Buttons: Test Connection & Save */}
          <div className="flex items-center gap-2 pt-1 flex-wrap">
            <button
              type="button"
              onClick={() => testConnection()}
              disabled={testingConn || savingConn}
              className="flex items-center gap-1.5 px-3 py-1.5 bg-secondary text-secondary-foreground hover:bg-secondary/80 border border-border rounded-none text-xs font-medium disabled:opacity-50"
            >
              {testingConn ? <Loader2 className="w-3.5 h-3.5 animate-spin text-primary" /> : <Zap className="w-3.5 h-3.5 text-yellow-500" />}
              <span>Test Connection</span>
            </button>

            <button
              type="button"
              onClick={addConnection}
              disabled={savingConn || testingConn}
              className="flex items-center gap-1.5 px-4 py-1.5 bg-primary text-primary-foreground rounded-none text-xs font-medium hover:bg-primary/90 disabled:opacity-50 ml-auto"
            >
              {savingConn ? <Loader2 className="w-3.5 h-3.5 animate-spin" /> : <Plus className="w-3.5 h-3.5" />}
              <span>Add Connection</span>
            </button>
          </div>
        </div>
      </RouterSection>

      {/* ── MODEL COMBOS ────────────────────────────────────────────────────── */}
      <RouterSection title="Model Combos" count={combos.length} expanded={!!expanded.combos} onToggle={() => toggle('combos')}>
        <p className="text-xs text-muted-foreground mb-2">
          Group multiple models together under a single combo name with smart routing strategies.
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
                    <button onClick={() => deleteCombo(c.id)} className="ml-auto text-destructive hover:text-destructive/80">
                      <Trash2 className="w-3.5 h-3.5" />
                    </button>
                  </div>
                  <div className="mt-1 flex flex-wrap gap-1">
                    {models.map(m => (
                      <span key={m} className="text-[11px] bg-muted border border-border px-1.5 py-0.5 rounded-none font-mono">
                        {m}
                      </span>
                    ))}
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
              <input
                value={newCombo.name}
                onChange={e => setNewCombo(c => ({ ...c, name: e.target.value }))}
                placeholder="best-coding"
                className="w-full border border-border rounded-none px-2 py-1.5 text-xs bg-background font-mono"
              />
            </div>
            <div>
              <label className="text-xs mb-1 block">Strategy</label>
              <select
                value={newCombo.strategy}
                onChange={e => setNewCombo(c => ({ ...c, strategy: e.target.value }))}
                className="w-full border border-border rounded-none px-2 py-1.5 text-xs bg-background"
              >
                {STRATEGIES.map(s => <option key={s.value} value={s.value}>{s.label}</option>)}
              </select>
            </div>
          </div>

          <div className="space-y-1.5">
            <div className="flex items-center justify-between">
              <label className="text-xs block">Models in Combo</label>
              {allDiscoveredModels.length > 0 && (
                <span className="text-[10px] text-muted-foreground">
                  Quick suggestions from connected providers available in dropdown
                </span>
              )}
            </div>

            {newCombo.models.map((m, i) => (
              <div key={i} className="flex gap-2">
                <input
                  value={m}
                  onChange={e => {
                    const models = [...newCombo.models];
                    models[i] = e.target.value;
                    setNewCombo(c => ({ ...c, models }));
                  }}
                  list={`combo-model-list-${i}`}
                  placeholder="e.g. gpt-4o, claude-3-5-sonnet, or llama3.3"
                  className="flex-1 border border-border rounded-none px-2 py-1.5 text-xs bg-background font-mono"
                />
                <datalist id={`combo-model-list-${i}`}>
                  {allDiscoveredModels.map(s => <option key={s} value={s} />)}
                </datalist>
                {newCombo.models.length > 1 && (
                  <button
                    onClick={() => setNewCombo(c => ({ ...c, models: c.models.filter((_, j) => j !== i) }))}
                    className="text-destructive px-2"
                  >
                    <Trash2 className="w-3.5 h-3.5" />
                  </button>
                )}
              </div>
            ))}

            <button
              onClick={() => setNewCombo(c => ({ ...c, models: [...c.models, ''] }))}
              className="text-xs text-primary flex items-center gap-1 mt-1 font-medium"
            >
              <Plus className="w-3 h-3" /> Add model
            </button>
          </div>

          <button
            onClick={saveCombo}
            disabled={savingCombo}
            className="flex items-center gap-1.5 px-3 py-1.5 bg-primary text-primary-foreground rounded-none text-xs font-medium"
          >
            {savingCombo ? <Loader2 className="w-3 h-3 animate-spin" /> : <Save className="w-3 h-3" />} Save Combo
          </button>
        </div>
      </RouterSection>

      {/* ── MODEL ALIASES ────────────────────────────────────────────────────── */}
      <RouterSection title="Model Aliases" count={aliases.length} expanded={!!expanded.aliases} onToggle={() => toggle('aliases')}>
        <p className="text-xs text-muted-foreground mb-2">
          Map custom alias names to specific target models or combos.
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
                <button onClick={() => deleteAlias(a.id)} className="text-destructive hover:text-destructive/80">
                  <Trash2 className="w-3.5 h-3.5" />
                </button>
              </div>
            ))}
          </div>
        )}

        <div className="border border-border rounded-none p-3 space-y-2.5 bg-card">
          <p className="text-[11px] font-semibold text-muted-foreground uppercase tracking-wide">Add Alias</p>
          <div className="flex gap-2 flex-wrap items-center">
            <input
              value={newAlias.alias_name}
              onChange={e => setNewAlias(a => ({ ...a, alias_name: e.target.value }))}
              placeholder="claude"
              className="border border-border rounded-none px-2 py-1.5 text-xs bg-background font-mono w-28"
            />
            <span className="self-center text-muted-foreground">→</span>
            <input
              value={newAlias.target_model}
              onChange={e => setNewAlias(a => ({ ...a, target_model: e.target.value }))}
              list="alias-target-list"
              placeholder="claude-3-5-sonnet, gpt-4o, or combo"
              className="flex-1 min-w-[180px] border border-border rounded-none px-2 py-1.5 text-xs bg-background font-mono"
            />
            <datalist id="alias-target-list">
              {allDiscoveredModels.map(s => <option key={s} value={s} />)}
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

            <button
              onClick={saveAlias}
              disabled={savingAlias}
              className="flex items-center gap-1.5 px-3 py-1.5 bg-primary text-primary-foreground rounded-none text-xs font-medium hover:bg-primary/90 disabled:opacity-50"
            >
              {savingAlias ? <Loader2 className="w-3 h-3 animate-spin" /> : <Plus className="w-3 h-3" />} Add
            </button>
          </div>
        </div>
      </RouterSection>

      {/* ── OPENWEBUI CHAT ACCESS ───────────────────────────────────────────── */}
      <div className="bg-card border border-border rounded-none p-4 space-y-3">
        <div className="flex items-center justify-between flex-wrap gap-2">
          <div>
            <h3 className="font-semibold text-sm flex items-center gap-1.5">
              <MessageSquare className="w-4 h-4 text-primary" /> OpenWebUI
            </h3>
            <p className="text-xs text-muted-foreground mt-0.5">
              Visual AI chat interface connected directly to your router.
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
