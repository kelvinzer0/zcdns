import React, { useState, useEffect, Suspense } from 'react';
import { Save, Copy, Check, Eye, EyeOff, Loader2, Network, PlayCircle } from 'lucide-react';
import { useToast } from '../../hooks/use-toast';

const AIRouterBlockly = React.lazy(() => import('./AIRouterBlockly').then(m => ({ default: m.AIRouterBlockly })));

interface AIRouterUIProps {
  subdomain: string;
}

export function AIRouterUI({ subdomain }: AIRouterUIProps) {
  const { toast } = useToast();
  
  const [loading, setLoading] = useState(true);
  const [saving, setSaving] = useState(false);
  const [testing, setTesting] = useState(false);
  
  const [inputFormat, setInputFormat] = useState('openai');
  const [outputFormat, setOutputFormat] = useState('openai');
  const [providerKeys, setProviderKeys] = useState({
    openai: '', anthropic: '', custom: '', custom_url: ''
  });
  const [routingRules, setRoutingRules] = useState<any[]>([]);
  
  // Test section
  const [testModel, setTestModel] = useState('gpt-4o');
  const [testResponse, setTestResponse] = useState('');

  // UI state
  const [copiedOpenAI, setCopiedOpenAI] = useState(false);
  const [copiedAnthropic, setCopiedAnthropic] = useState(false);
  const [showKeys, setShowKeys] = useState(false);

  useEffect(() => {
    fetchConfig();
  }, [subdomain]);

  const fetchConfig = async () => {
    try {
      const response = await fetch(`/api/airouter/config`, {
        headers: { 'X-Subdomain': subdomain }
      });
      if (response.ok) {
        const res = await response.json();
        setInputFormat(res.input_format || 'openai');
        setOutputFormat(res.output_format || 'openai');
        try {
          setRoutingRules(JSON.parse(res.routing_rules_json || '[]'));
          setProviderKeys(JSON.parse(res.provider_keys_json || '{}'));
        } catch (e) {}
      }
    } catch (err: any) {
      toast({ title: 'Error loading config', description: err.message, variant: 'destructive' });
    } finally {
      setLoading(false);
    }
  };

  const handleSave = async (rulesToSave?: any[]) => {
    setSaving(true);
    try {
      const response = await fetch(`/api/airouter/config`, {
        method: 'POST',
        headers: {
          'Content-Type': 'application/json',
          'X-Subdomain': subdomain
        },
        body: JSON.stringify({
          input_format: inputFormat,
          output_format: outputFormat,
          routing_rules_json: JSON.stringify(rulesToSave || routingRules),
          provider_keys_json: JSON.stringify(providerKeys)
        })
      });
      if (!response.ok) {
        throw new Error('Failed to save configuration');
      }
      toast({ title: 'Saved successfully', description: 'AI Router configuration updated.' });
    } catch (err: any) {
      toast({ title: 'Save failed', description: err.message, variant: 'destructive' });
    } finally {
      setSaving(false);
    }
  };

  const copyToClipboard = (text: string, type: 'openai' | 'anthropic') => {
    navigator.clipboard.writeText(text);
    if (type === 'openai') {
      setCopiedOpenAI(true);
      setTimeout(() => setCopiedOpenAI(false), 2000);
    } else {
      setCopiedAnthropic(true);
      setTimeout(() => setCopiedAnthropic(false), 2000);
    }
  };

  const handleTest = async () => {
    setTesting(true);
    setTestResponse('Routing...');
    try {
      const res = await fetch(`https://${subdomain}.router.zcdns.id/v1/chat/completions`, {
        method: 'POST',
        headers: { 'Content-Type': 'application/json', 'Authorization': 'Bearer test' },
        body: JSON.stringify({
          model: testModel,
          messages: [{ role: 'user', content: 'Say "Hello, ZCDNS Router is working!"' }]
        })
      });
      const data = await res.json();
      setTestResponse(JSON.stringify(data, null, 2));
    } catch (err: any) {
      setTestResponse('Error: ' + err.message + '\nMake sure you have saved your keys and wildcard DNS is active.');
    } finally {
      setTesting(false);
    }
  };

  if (loading) {
    return (
      <div className="flex justify-center items-center h-64">
        <Loader2 className="w-8 h-8 animate-spin text-primary" />
      </div>
    );
  }

  return (
    <div className="space-y-6">
      <div className="flex items-center justify-between">
        <div>
          <h2 className="text-2xl font-bold tracking-tight flex items-center gap-2">
            <Network className="w-6 h-6 text-primary" />
            AI Router (Preview)
          </h2>
          <p className="text-muted-foreground mt-1">
            Connect Claude Code, Cursor, Cline, or Copilot to free AI providers, setup fallbacks, and optimize token usage.
          </p>
        </div>
        <button
          onClick={() => handleSave()}
          disabled={saving}
          className="flex items-center gap-2 px-4 py-2 bg-primary text-primary-foreground rounded-md hover:bg-primary/90 transition-colors"
        >
          {saving ? <Loader2 className="w-4 h-4 animate-spin" /> : <Save className="w-4 h-4" />}
          Save Configuration
        </button>
      </div>

      {/* Connection Info */}
      <div className="bg-card border border-border rounded-lg p-5">
        <h3 className="text-lg font-semibold mb-4 border-b border-border pb-2">Your Personal Proxy Endpoints</h3>
        <p className="text-sm text-muted-foreground mb-4">
          Use these URLs as the "Base URL" in your favorite AI coding tool. The proxy routes requests based on your Blockly logic below.
          <br/><strong>Note:</strong> Authentication is handled via your subdomain. You can use any string (e.g. `zcdns-key`) as the API key in your tool.
        </p>
        
        <div className="space-y-4">
          <div>
            <label className="text-sm font-medium">OpenAI Compatible Endpoint</label>
            <div className="flex mt-1">
              <input 
                type="text" 
                readOnly 
                value={`https://${subdomain}.router.zcdns.id/v1/chat/completions`}
                className="flex-1 bg-muted px-3 py-2 rounded-l-md border border-border font-mono text-sm"
              />
              <button 
                onClick={() => copyToClipboard(`https://${subdomain}.router.zcdns.id/v1/chat/completions`, 'openai')}
                className="px-3 bg-secondary border border-l-0 border-border rounded-r-md hover:bg-secondary/80 flex items-center justify-center w-12"
              >
                {copiedOpenAI ? <Check className="w-4 h-4 text-green-500" /> : <Copy className="w-4 h-4" />}
              </button>
            </div>
          </div>

          <div>
            <label className="text-sm font-medium">Anthropic Compatible Endpoint</label>
            <div className="flex mt-1">
              <input 
                type="text" 
                readOnly 
                value={`https://${subdomain}.router.zcdns.id/v1/messages`}
                className="flex-1 bg-muted px-3 py-2 rounded-l-md border border-border font-mono text-sm"
              />
              <button 
                onClick={() => copyToClipboard(`https://${subdomain}.router.zcdns.id/v1/messages`, 'anthropic')}
                className="px-3 bg-secondary border border-l-0 border-border rounded-r-md hover:bg-secondary/80 flex items-center justify-center w-12"
              >
                {copiedAnthropic ? <Check className="w-4 h-4 text-green-500" /> : <Copy className="w-4 h-4" />}
              </button>
            </div>
          </div>
        </div>
      </div>

      {/* Provider Keys */}
      <div className="bg-card border border-border rounded-lg p-5">
        <div className="flex justify-between items-center mb-4 border-b border-border pb-2">
          <h3 className="text-lg font-semibold">Provider API Keys</h3>
          <button 
            onClick={() => setShowKeys(!showKeys)}
            className="text-sm flex items-center gap-1 text-muted-foreground hover:text-foreground"
          >
            {showKeys ? <><EyeOff className="w-4 h-4"/> Hide Keys</> : <><Eye className="w-4 h-4"/> Show Keys</>}
          </button>
        </div>
        
        <div className="grid grid-cols-1 md:grid-cols-2 gap-4">
          <div>
            <label className="text-sm font-medium block mb-1">OpenAI API Key</label>
            <input 
              type={showKeys ? "text" : "password"} 
              placeholder="sk-..."
              value={providerKeys.openai || ''}
              onChange={e => setProviderKeys({...providerKeys, openai: e.target.value})}
              className="w-full bg-background px-3 py-2 rounded-md border border-border"
            />
          </div>
          <div>
            <label className="text-sm font-medium block mb-1">Anthropic API Key</label>
            <input 
              type={showKeys ? "text" : "password"} 
              placeholder="sk-ant-..."
              value={providerKeys.anthropic || ''}
              onChange={e => setProviderKeys({...providerKeys, anthropic: e.target.value})}
              className="w-full bg-background px-3 py-2 rounded-md border border-border"
            />
          </div>
          <div className="md:col-span-2 border border-border rounded p-3 bg-muted/30">
            <label className="text-sm font-medium block mb-2 text-muted-foreground">Custom Provider (Groq, Together, Ollama, etc)</label>
            <div className="grid grid-cols-1 md:grid-cols-2 gap-4">
              <div>
                <label className="text-xs block mb-1">Base URL</label>
                <input 
                  type="text" 
                  placeholder="https://api.groq.com/openai"
                  value={providerKeys.custom_url || ''}
                  onChange={e => setProviderKeys({...providerKeys, custom_url: e.target.value})}
                  className="w-full bg-background px-3 py-2 rounded-md border border-border text-sm"
                />
              </div>
              <div>
                <label className="text-xs block mb-1">API Key</label>
                <input 
                  type={showKeys ? "text" : "password"} 
                  placeholder="Custom Key..."
                  value={providerKeys.custom || ''}
                  onChange={e => setProviderKeys({...providerKeys, custom: e.target.value})}
                  className="w-full bg-background px-3 py-2 rounded-md border border-border text-sm"
                />
              </div>
            </div>
          </div>
        </div>
      </div>

      {/* Routing Logic Blockly */}
      <div className="bg-card border border-border rounded-lg p-5 flex flex-col h-[600px]">
        <h3 className="text-lg font-semibold mb-2">Routing Logic</h3>
        <p className="text-sm text-muted-foreground mb-4">
          Drag and drop blocks to build your custom AI router. Rules are evaluated from top to bottom.
        </p>
        <div className="flex-1 bg-white dark:bg-zinc-800 rounded-md overflow-hidden border border-border relative">
          <Suspense fallback={<div className="absolute inset-0 flex items-center justify-center bg-background/50 backdrop-blur-sm"><div className="flex flex-col items-center"><Loader2 className="w-8 h-8 animate-spin text-primary mb-2" /><p className="text-sm font-medium">Loading Router Editor...</p></div></div>}>
             <AIRouterBlockly 
                initialRules={routingRules} 
                onChange={(rules: any[]) => {
                  setRoutingRules(rules);
                }} 
             />
          </Suspense>
        </div>
      </div>

      {/* Quick Test */}
      <div className="bg-card border border-border rounded-lg p-5">
        <h3 className="text-lg font-semibold mb-4 border-b border-border pb-2">Quick Test</h3>
        <div className="flex gap-2 mb-4">
          <input 
            type="text" 
            placeholder="Model (e.g. gpt-4o, claude-3-sonnet)"
            value={testModel}
            onChange={e => setTestModel(e.target.value)}
            className="flex-1 bg-background px-3 py-2 rounded-md border border-border"
          />
          <button 
            onClick={handleTest}
            disabled={testing}
            className="px-4 py-2 bg-secondary text-secondary-foreground rounded-md hover:bg-secondary/80 flex items-center gap-2"
          >
            {testing ? <Loader2 className="w-4 h-4 animate-spin" /> : <PlayCircle className="w-4 h-4" />}
            Test Route
          </button>
        </div>
        <pre className="bg-muted p-4 rounded-md overflow-x-auto text-xs font-mono border border-border min-h-[100px] whitespace-pre-wrap">
          {testResponse || 'Test response will appear here...'}
        </pre>
      </div>

    </div>
  );
}
