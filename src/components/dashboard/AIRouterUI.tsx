import React, { useState, useEffect, useCallback } from 'react';
import {
  Network, Plus, Trash2, Save, Copy, Check, Eye, EyeOff,
  Loader2, ChevronDown, ChevronRight, PlayCircle, AlertCircle, CheckCircle2
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
}

interface Config {
  input_format: string;
  output_format: string;
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
  { value: 'fallback', label: 'Fallback — try in order, next if fail' },
  { value: 'round_robin', label: 'Round Robin — rotate each request' },
  { value: 'round_robin_sticky', label: 'Round Robin Sticky — rotate per session' },
];

const MODEL_SUGGESTIONS: Record<string, string[]> = {
  openai: ['openai/gpt-4o', 'openai/gpt-4o-mini', 'openai/o1-mini'],
  anthropic: ['anthropic/claude-3-5-sonnet-20241022', 'anthropic/claude-3-haiku-20240307'],
  groq: ['groq/llama-3.3-70b-versatile', 'groq/llama-3.1-8b-instant', 'groq/gemma2-9b-it'],
  together: ['together/meta-llama/Llama-3-70b-chat-hf'],
  openrouter: ['openrouter/anthropic/claude-3.5-sonnet', 'openrouter/openai/gpt-4o'],
};

export function AIRouterUI({ subdomain }: { subdomain: string }) {
  const { toast } = useToast();
  const [loading, setLoading] = useState(true);

  const [config, setConfig] = useState<Config>({ input_format: 'auto', output_format: 'openai' });
  const [connections, setConnections] = useState<Connection[]>([]);
  const [combos, setCombos] = useState<Combo[]>([]);
  const [aliases, setAliases] = useState<Alias[]>([]);

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
  const [newAlias, setNewAlias] = useState({ alias_name: '', target_model: '' });
  const [savingAlias, setSavingAlias] = useState(false);

  // Quick test
  const [testModel, setTestModel] = useState('gpt-4o');
  const [testMsg, setTestMsg] = useState('Say hello in one sentence.');
  const [testing, setTesting] = useState(false);
  const [testResult, setTestResult] = useState('');

  const fetchAll = useCallback(async () => {
    try {
      const res = await fetch('/api/airouter/config', {
        headers: { 'X-Subdomain': subdomain }
      });
      if (!res.ok) throw new Error(await res.text());
      const data = await res.json();
      setConfig(data.config || { input_format: 'auto', output_format: 'openai' });
      setConnections(data.connections || []);
      setCombos(data.combos || []);
      setAliases(data.aliases || []);
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

  const saveConfig = async () => {
    await fetch('/api/airouter/config', {
      method: 'POST',
      headers: { 'Content-Type': 'application/json', 'X-Subdomain': subdomain },
      body: JSON.stringify(config)
    });
    toast({ title: 'Config saved' });
  };

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
      setNewAlias({ alias_name: '', target_model: '' });
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

  const runTest = async () => {
    setTesting(true);
    setTestResult('Routing request...');
    try {
      const endpoint = `https://${subdomain}.router.zcdns.id/v1/chat/completions`;
      const res = await fetch(endpoint, {
        method: 'POST',
        headers: { 'Content-Type': 'application/json', 'Authorization': 'Bearer zcdns-test' },
        body: JSON.stringify({
          model: testModel,
          messages: [{ role: 'user', content: testMsg }],
          stream: false
        })
      });
      const data = await res.json();
      const provider = res.headers.get('x-router-provider') || '?';
      const model = res.headers.get('x-router-model') || '?';
      setTestResult(`→ Routed to: ${provider}/${model}\n\n${JSON.stringify(data, null, 2)}`);
    } catch (e: any) {
      setTestResult('Error: ' + e.message + '\n\nMake sure wildcard SSL is active and at least one connection is configured.');
    } finally {
      setTesting(false);
    }
  };

  if (loading) return (
    <div className="flex justify-center items-center h-64">
      <Loader2 className="w-8 h-8 animate-spin text-primary" />
    </div>
  );

  const openaiUrl = `https://${subdomain}.router.zcdns.id/v1/chat/completions`;
  const anthropicUrl = `https://${subdomain}.router.zcdns.id/v1/messages`;

  const Section = ({ id, title, count, children }: { id: string, title: string, count?: number, children: React.ReactNode }) => (
    <div className="bg-card border border-border rounded-lg overflow-hidden">
      <button onClick={() => toggle(id)} className="w-full flex items-center justify-between px-5 py-4 hover:bg-muted/30 transition-colors">
        <div className="flex items-center gap-2 font-semibold">
          {expanded[id] ? <ChevronDown className="w-4 h-4" /> : <ChevronRight className="w-4 h-4" />}
          {title}
          {count !== undefined && <span className="text-xs bg-primary/10 text-primary px-2 py-0.5 rounded-full">{count}</span>}
        </div>
      </button>
      {expanded[id] && <div className="px-5 pb-5">{children}</div>}
    </div>
  );

  return (
    <div className="space-y-5">
      {/* Header */}
      <div className="flex items-center justify-between">
        <div>
          <h2 className="text-2xl font-bold tracking-tight flex items-center gap-2">
            <Network className="w-6 h-6 text-primary" /> AI Router
          </h2>
          <p className="text-sm text-muted-foreground mt-1">
            Personal AI proxy with multi-provider fallback, combos, and model aliases.
          </p>
        </div>
      </div>

      {/* Endpoint URLs */}
      <div className="bg-card border border-border rounded-lg p-5">
        <h3 className="font-semibold mb-3">Your Proxy Endpoints</h3>
        <p className="text-xs text-muted-foreground mb-3">Use any string as API key in your tool — auth is by subdomain.</p>
        {[
          { label: 'OpenAI-compatible', url: openaiUrl },
          { label: 'Anthropic-compatible', url: anthropicUrl },
        ].map(({ label, url }) => (
          <div key={url} className="mb-2">
            <span className="text-xs text-muted-foreground">{label}</span>
            <div className="flex mt-1">
              <input readOnly value={url} className="flex-1 bg-muted px-3 py-2 text-sm font-mono rounded-l-md border border-border" />
              <button onClick={() => copyUrl(url)} className="px-3 border border-l-0 border-border rounded-r-md hover:bg-muted flex items-center">
                {copiedUrl === url ? <Check className="w-4 h-4 text-green-500" /> : <Copy className="w-4 h-4" />}
              </button>
            </div>
          </div>
        ))}
        <div className="mt-4 flex items-center gap-4 flex-wrap">
          <label className="text-sm font-medium">Input Format</label>
          <select value={config.input_format} onChange={e => setConfig(c => ({ ...c, input_format: e.target.value }))}
            className="border border-border rounded px-2 py-1 text-sm bg-background">
            <option value="auto">Auto-detect</option>
            <option value="openai">Force OpenAI</option>
            <option value="anthropic">Force Anthropic</option>
          </select>
          <label className="text-sm font-medium">Output Format</label>
          <select value={config.output_format} onChange={e => setConfig(c => ({ ...c, output_format: e.target.value }))}
            className="border border-border rounded px-2 py-1 text-sm bg-background">
            <option value="openai">OpenAI</option>
            <option value="anthropic">Anthropic</option>
          </select>
          <button onClick={saveConfig} className="px-3 py-1.5 bg-primary text-primary-foreground rounded text-sm flex items-center gap-1">
            <Save className="w-3.5 h-3.5" /> Save
          </button>
        </div>
      </div>

      {/* Connections */}
      <Section id="connections" title="Provider Connections" count={connections.length}>
        <div className="flex justify-between items-center mb-3">
          <p className="text-sm text-muted-foreground">Add multiple API keys per provider. When one is rate-limited, next is auto-selected.</p>
          <button onClick={() => setShowKeys(!showKeys)} className="text-xs flex items-center gap-1 text-muted-foreground">
            {showKeys ? <><EyeOff className="w-3 h-3" /> Hide</> : <><Eye className="w-3 h-3" /> Show Keys</>}
          </button>
        </div>

        {/* Existing connections */}
        {connections.length > 0 && (
          <div className="mb-4 space-y-2">
            {connections.map(c => (
              <div key={c.id} className="flex items-center gap-2 p-3 bg-muted/40 rounded border border-border text-sm">
                <span className="font-mono text-xs bg-primary/10 text-primary px-2 py-0.5 rounded">{c.provider}</span>
                <span className="font-medium flex-1">{c.name}</span>
                <span className="font-mono text-muted-foreground text-xs">{showKeys ? c.api_key : c.api_key}</span>
                {c.status === 'active'
                  ? <CheckCircle2 className="w-4 h-4 text-green-500 shrink-0" />
                  : <span title="Rate limited"><AlertCircle className="w-4 h-4 text-yellow-500 shrink-0" /></span>}
                <button onClick={() => deleteConnection(c.id)} className="text-destructive hover:text-destructive/80 shrink-0">
                  <Trash2 className="w-4 h-4" />
                </button>
              </div>
            ))}
          </div>
        )}

        {/* Add new connection form */}
        <div className="border border-dashed border-border rounded-lg p-4 space-y-3">
          <p className="text-xs font-medium text-muted-foreground uppercase tracking-wide">Add Connection</p>
          <div className="grid grid-cols-1 sm:grid-cols-2 gap-3">
            <div>
              <label className="text-xs mb-1 block">Provider</label>
              <select value={newConn.provider} onChange={e => setNewConn(c => ({ ...c, provider: e.target.value }))}
                className="w-full border border-border rounded px-2 py-1.5 text-sm bg-background">
                {PROVIDERS.map(p => <option key={p.value} value={p.value}>{p.label}</option>)}
              </select>
            </div>
            <div>
              <label className="text-xs mb-1 block">Label (e.g. "Personal Key")</label>
              <input value={newConn.name} onChange={e => setNewConn(c => ({ ...c, name: e.target.value }))}
                placeholder="My OpenAI key" className="w-full border border-border rounded px-2 py-1.5 text-sm bg-background" />
            </div>
            <div>
              <label className="text-xs mb-1 block">API Key</label>
              <input type="password" value={newConn.api_key} onChange={e => setNewConn(c => ({ ...c, api_key: e.target.value }))}
                placeholder="sk-..." className="w-full border border-border rounded px-2 py-1.5 text-sm bg-background font-mono" />
            </div>
            {newConn.provider === 'custom' && (
              <div>
                <label className="text-xs mb-1 block">Base URL</label>
                <input value={newConn.base_url} onChange={e => setNewConn(c => ({ ...c, base_url: e.target.value }))}
                  placeholder="https://localhost:11434" className="w-full border border-border rounded px-2 py-1.5 text-sm bg-background" />
              </div>
            )}
          </div>
          <button onClick={addConnection} disabled={savingConn}
            className="flex items-center gap-2 px-3 py-1.5 bg-primary text-primary-foreground rounded text-sm">
            {savingConn ? <Loader2 className="w-3.5 h-3.5 animate-spin" /> : <Plus className="w-3.5 h-3.5" />} Add Connection
          </button>
        </div>
      </Section>

      {/* Combos */}
      <Section id="combos" title="Combos" count={combos.length}>
        <p className="text-sm text-muted-foreground mb-3">
          Group models into a combo. Reference a combo by name (e.g. <code className="bg-muted px-1 rounded">best-coding</code>) as the model in your tool.
        </p>

        {combos.length > 0 && (
          <div className="mb-4 space-y-2">
            {combos.map(c => {
              const models: string[] = JSON.parse(c.models_json || '[]');
              return (
                <div key={c.id} className="p-3 bg-muted/40 rounded border border-border text-sm">
                  <div className="flex items-center gap-2">
                    <code className="font-mono font-bold text-primary">{c.name}</code>
                    <span className="text-xs bg-secondary px-2 py-0.5 rounded">{c.strategy}</span>
                    <button onClick={() => deleteCombo(c.id)} className="ml-auto text-destructive">
                      <Trash2 className="w-4 h-4" />
                    </button>
                  </div>
                  <div className="mt-1 flex flex-wrap gap-1">
                    {models.map(m => <span key={m} className="text-xs bg-muted border border-border px-2 py-0.5 rounded font-mono">{m}</span>)}
                  </div>
                </div>
              );
            })}
          </div>
        )}

        <div className="border border-dashed border-border rounded-lg p-4 space-y-3">
          <p className="text-xs font-medium text-muted-foreground uppercase tracking-wide">Create Combo</p>
          <div className="grid grid-cols-1 sm:grid-cols-2 gap-3">
            <div>
              <label className="text-xs mb-1 block">Combo Name</label>
              <input value={newCombo.name} onChange={e => setNewCombo(c => ({ ...c, name: e.target.value }))}
                placeholder="best-coding" className="w-full border border-border rounded px-2 py-1.5 text-sm bg-background font-mono" />
            </div>
            <div>
              <label className="text-xs mb-1 block">Strategy</label>
              <select value={newCombo.strategy} onChange={e => setNewCombo(c => ({ ...c, strategy: e.target.value }))}
                className="w-full border border-border rounded px-2 py-1.5 text-sm bg-background">
                {STRATEGIES.map(s => <option key={s.value} value={s.value}>{s.label}</option>)}
              </select>
            </div>
          </div>
          <div className="space-y-2">
            <label className="text-xs block">Models (in order of priority)</label>
            {newCombo.models.map((m, i) => (
              <div key={i} className="flex gap-2">
                <input value={m} onChange={e => {
                  const models = [...newCombo.models];
                  models[i] = e.target.value;
                  setNewCombo(c => ({ ...c, models }));
                }}
                  list={`model-suggestions-${i}`}
                  placeholder="openai/gpt-4o or groq/llama-3.3-70b-versatile"
                  className="flex-1 border border-border rounded px-2 py-1.5 text-sm bg-background font-mono" />
                <datalist id={`model-suggestions-${i}`}>
                  {Object.values(MODEL_SUGGESTIONS).flat().map(s => <option key={s} value={s} />)}
                </datalist>
                {newCombo.models.length > 1 && (
                  <button onClick={() => setNewCombo(c => ({ ...c, models: c.models.filter((_, j) => j !== i) }))}
                    className="text-destructive px-2"><Trash2 className="w-4 h-4" /></button>
                )}
              </div>
            ))}
            <button onClick={() => setNewCombo(c => ({ ...c, models: [...c.models, ''] }))}
              className="text-xs text-primary flex items-center gap-1 mt-1">
              <Plus className="w-3.5 h-3.5" /> Add model
            </button>
          </div>
          <button onClick={saveCombo} disabled={savingCombo}
            className="flex items-center gap-2 px-3 py-1.5 bg-primary text-primary-foreground rounded text-sm">
            {savingCombo ? <Loader2 className="w-3.5 h-3.5 animate-spin" /> : <Save className="w-3.5 h-3.5" />} Save Combo
          </button>
        </div>
      </Section>

      {/* Aliases */}
      <Section id="aliases" title="Model Aliases" count={aliases.length}>
        <p className="text-sm text-muted-foreground mb-3">
          Map any model name to a real <code className="bg-muted px-1 rounded">provider/model</code> or a combo name.
          E.g. <code className="bg-muted px-1 rounded">claude</code> → <code className="bg-muted px-1 rounded">anthropic/claude-3-5-sonnet-20241022</code>
        </p>

        {aliases.length > 0 && (
          <div className="mb-4 space-y-2">
            {aliases.map(a => (
              <div key={a.id} className="flex items-center gap-3 p-3 bg-muted/40 rounded border border-border text-sm">
                <code className="font-mono font-bold text-primary">{a.alias_name}</code>
                <span className="text-muted-foreground">→</span>
                <code className="font-mono text-sm flex-1">{a.target_model}</code>
                <button onClick={() => deleteAlias(a.id)} className="text-destructive">
                  <Trash2 className="w-4 h-4" />
                </button>
              </div>
            ))}
          </div>
        )}

        <div className="border border-dashed border-border rounded-lg p-4 space-y-3">
          <p className="text-xs font-medium text-muted-foreground uppercase tracking-wide">Add Alias</p>
          <div className="flex gap-3 flex-wrap">
            <input value={newAlias.alias_name} onChange={e => setNewAlias(a => ({ ...a, alias_name: e.target.value }))}
              placeholder="claude" className="border border-border rounded px-2 py-1.5 text-sm bg-background font-mono w-40" />
            <span className="self-center text-muted-foreground">→</span>
            <input value={newAlias.target_model} onChange={e => setNewAlias(a => ({ ...a, target_model: e.target.value }))}
              list="alias-target-list"
              placeholder="anthropic/claude-3-5-sonnet-20241022 or combo-name"
              className="flex-1 border border-border rounded px-2 py-1.5 text-sm bg-background font-mono" />
            <datalist id="alias-target-list">
              {Object.values(MODEL_SUGGESTIONS).flat().map(s => <option key={s} value={s} />)}
              {combos.map(c => <option key={c.name} value={c.name} />)}
            </datalist>
            <button onClick={saveAlias} disabled={savingAlias}
              className="flex items-center gap-2 px-3 py-1.5 bg-primary text-primary-foreground rounded text-sm">
              {savingAlias ? <Loader2 className="w-3.5 h-3.5 animate-spin" /> : <Plus className="w-3.5 h-3.5" />} Add
            </button>
          </div>
        </div>
      </Section>

      {/* Quick Test */}
      <div className="bg-card border border-border rounded-lg p-5">
        <h3 className="font-semibold mb-3">Quick Test</h3>
        <div className="flex gap-2 mb-3 flex-wrap">
          <input value={testModel} onChange={e => setTestModel(e.target.value)}
            placeholder="Model or alias or combo name" className="border border-border rounded px-2 py-1.5 text-sm bg-background font-mono w-56" />
          <input value={testMsg} onChange={e => setTestMsg(e.target.value)}
            placeholder="Message..." className="flex-1 border border-border rounded px-2 py-1.5 text-sm bg-background" />
          <button onClick={runTest} disabled={testing}
            className="flex items-center gap-2 px-3 py-1.5 bg-secondary text-secondary-foreground rounded text-sm">
            {testing ? <Loader2 className="w-3.5 h-3.5 animate-spin" /> : <PlayCircle className="w-3.5 h-3.5" />} Test
          </button>
        </div>
        <pre className="bg-muted p-4 rounded border border-border text-xs font-mono whitespace-pre-wrap min-h-[80px] max-h-[300px] overflow-auto">
          {testResult || 'Response will appear here...'}
        </pre>
      </div>
    </div>
  );
}
