import { useState, useEffect } from 'react';
import { 
  GitCommit, 
  Key, 
  Shield, 
  ChevronDown, 
  Download, 
  Terminal, 
  Copy, 
  Check, 
  X, 
  ExternalLink,
  Laptop,
  Apple,
  Smartphone,
  Eye,
  EyeOff
} from 'lucide-react';
import { Button } from '../ui/button';

export function VaultUI({ subdomain }: { subdomain: string }) {
  const [branches, setBranches] = useState<any[]>([]);
  const [commits, setCommits] = useState<any[]>([]);
  const [secrets, setSecrets] = useState<any[]>([]);
  
  const [selectedBranch, setSelectedBranch] = useState('main');
  const [selectedCommitIdx, setSelectedCommitIdx] = useState(0);
  
  const [showDownloadModal, setShowDownloadModal] = useState(false);
  const [copiedCmd, setCopiedCmd] = useState<string | null>(null);
  const [revealed, setRevealed] = useState<Record<string, boolean>>({});
  const [repo, setRepo] = useState<any>(null);
  
  const [dlOS, setDlOS] = useState('Linux');
  const [dlArch, setDlArch] = useState(0);

  useEffect(() => {
    fetch('/api/vault/init', {
      method: 'POST',
      headers: { 'X-Subdomain': subdomain }
    }).then(r => r.json()).then(data => {
      setRepo(data);
    });
  }, [subdomain]);

  useEffect(() => {
    if (!repo) return;
    fetch(`/api/vault/branches?repo_id=${repo.id}`, {
      headers: { 'X-Subdomain': subdomain }
    }).then(r => r.json()).then(data => {
      setBranches(data || []);
      if (data && data.length > 0) {
        setSelectedBranch(data[0].name);
      }
    });
    
    fetch(`/api/vault/commits?repo_id=${repo.id}`, {
      headers: { 'X-Subdomain': subdomain }
    }).then(r => r.json()).then(data => {
      setCommits(data || []);
      if (data && data.length > 0) {
        setSelectedCommitIdx(0);
      }
    });
  }, [repo, subdomain]);

  useEffect(() => {
    if (commits.length > 0 && commits[selectedCommitIdx]) {
      fetch(`/api/vault/kv?commit=${commits[selectedCommitIdx].hash}`, {
        headers: { 'X-Subdomain': subdomain }
      }).then(r => r.json()).then(data => {
        setSecrets(data || []);
      });
    } else {
      setSecrets([]);
    }
  }, [commits, selectedCommitIdx, subdomain]);

  const handleRevertCommit = async () => {
    if (!repo || commits.length === 0 || !commits[selectedCommitIdx]) return;
    const commit = commits[selectedCommitIdx];
    await fetch('/api/vault/revert', {
      method: 'POST',
      headers: { 
        'X-Subdomain': subdomain,
        'Content-Type': 'application/json'
      },
      body: JSON.stringify({
        repo_id: repo.id,
        commit_hash: commit.hash
      })
    });
    // refresh commits
    fetch(`/api/vault/commits?repo_id=${repo.id}`, {
      headers: { 'X-Subdomain': subdomain }
    }).then(r => r.json()).then(data => {
      setCommits(data || []);
      setSelectedCommitIdx(0);
    });
  };

  const downloadOptions = [
    {
      os: 'Linux',
      icon: Laptop,
      archs: [
        { label: 'x86_64 (amd64)', filename: 'zvault-linux-amd64', url: '/bin/zvault-linux-amd64' },
        { label: 'ARM64 (aarch64)', filename: 'zvault-linux-arm64', url: '/bin/zvault-linux-arm64' },
      ],
      getInstallCmd: (archFile: string) => `mkdir -p ~/.local/bin && curl -fsSL https://zcdns.id/bin/${archFile} -o ~/.local/bin/zvault && chmod +x ~/.local/bin/zvault`
    },
    {
      os: 'macOS',
      icon: Apple,
      archs: [
        { label: 'Apple Silicon (M-series / ARM64)', filename: 'zvault-macos-arm64', url: '/bin/zvault-macos-arm64' },
        { label: 'Intel (x86_64)', filename: 'zvault-macos-amd64', url: '/bin/zvault-macos-amd64' },
      ],
      getInstallCmd: (archFile: string) => `mkdir -p ~/.local/bin && curl -fsSL https://zcdns.id/bin/${archFile} -o ~/.local/bin/zvault && chmod +x ~/.local/bin/zvault`
    },
    {
      os: 'Windows',
      icon: Laptop,
      archs: [
        { label: 'Windows 64-bit (x86_64 .exe)', filename: 'zvault-windows-amd64.exe', url: '/bin/zvault-windows-amd64.exe' },
      ],
      getInstallCmd: (archFile: string) => `curl.exe -fsSL https://zcdns.id/bin/${archFile} -o zvault.exe`
    },
    {
      os: 'Android (Termux)',
      icon: Smartphone,
      archs: [
        { label: 'ARM64 / aarch64 (Most devices)', filename: 'zvault-android-arm64', url: '/bin/zvault-android-arm64' },
        { label: 'ARM 32-bit (Older devices)', filename: 'zvault-linux-arm', url: '/bin/zvault-linux-arm' },
      ],
      getInstallCmd: (archFile: string) => `mkdir -p ~/.local/bin && curl -fsSL https://zcdns.id/bin/${archFile} -o ~/.local/bin/zvault && chmod +x ~/.local/bin/zvault`
    }
  ];

  const handleCopy = (text: string, id: string) => {
    navigator.clipboard.writeText(text);
    setCopiedCmd(id);
    setTimeout(() => setCopiedCmd(null), 2000);
  };

  const toggleReveal = (key: string) => {
    setRevealed(prev => ({ ...prev, [key]: !prev[key] }));
  };

  return (
    <div className="space-y-6 mt-6">
      {/* CLI Quick Download & Banner */}
      <div className="bg-gradient-to-r from-gray-900 via-gray-800 to-gray-900 text-white rounded-none border border-gray-800 p-4 shadow-sm">
        <div className="flex flex-col md:flex-row items-start md:items-center justify-between gap-4">
          <div className="flex items-start space-x-3">
            <div className="p-2 bg-green-500/10 border border-green-500/20 text-green-400 rounded-none">
              <Terminal className="w-5 h-5" />
            </div>
            <div>
              <div className="flex items-center gap-2">
                <span className="font-bold text-sm text-white">zvault CLI Available</span>
                <span className="text-[10px] uppercase font-mono px-1.5 py-0.5 bg-green-950 text-green-400 border border-green-800 rounded">v0.1.0</span>
              </div>
              <p className="text-xs text-gray-300 mt-0.5">
                Manage your secrets using Git-style workflows directly from your terminal. Supports Linux, macOS, and Windows.
              </p>
            </div>
          </div>
          <div className="flex items-center gap-2 w-full md:w-auto">
            <Button 
              onClick={() => setShowDownloadModal(true)}
              className="bg-green-600 hover:bg-green-500 text-white text-xs font-semibold px-4 py-2 h-auto rounded-none shadow flex items-center gap-1.5 w-full md:w-auto justify-center"
            >
              <Download className="w-4 h-4" />
              Download zvault CLI
            </Button>
          </div>
        </div>
      </div>

      {/* Main Vault Workspace */}
      <div className="bg-white rounded-none border border-gray-200 shadow-sm overflow-hidden">
        {/* Header Bar */}
        <div className="border-b border-gray-200 bg-gray-50 px-4 py-3 flex flex-wrap items-center justify-between gap-3">
          <div className="flex items-center space-x-2">
            <Shield className="w-5 h-5 text-gray-700" />
            <h2 className="text-lg font-bold text-gray-900">Secrets Vault</h2>
            
          </div>

          <div className="flex items-center space-x-3">
            <div className="flex items-center space-x-2">
              <span className="text-xs text-gray-500 font-medium">Branch:</span>
              <div className="relative">
                <select
                  value={selectedBranch}
                  onChange={(e) => setSelectedBranch(e.target.value)}
                  className="appearance-none bg-white border border-gray-300 text-gray-700 text-xs py-1.5 pl-2.5 pr-7 rounded-none focus:outline-none focus:ring-1 focus:ring-green-500 font-medium cursor-pointer"
                >
                  {branches.map(b => <option key={b.name} value={b.name}>{b.name}</option>)}
                </select>
                <ChevronDown className="w-3.5 h-3.5 text-gray-500 absolute right-2 top-1/2 transform -translate-y-1/2 pointer-events-none" />
              </div>
            </div>
          </div>
        </div>
        
        {/* Two-Column Explorer */}
        <div className="flex flex-col md:flex-row min-h-[550px] md:h-[550px]">
          {/* Left: Commit History */}
          <div className="w-full md:w-1/3 border-b md:border-b-0 md:border-r border-gray-200 bg-white overflow-y-auto max-h-[250px] md:max-h-none">
            <div className="px-4 py-2.5 border-b border-gray-200 bg-gray-50 sticky top-0 flex items-center justify-between z-10">
              <h3 className="text-xs font-bold uppercase tracking-wider text-gray-600">Commit History</h3>
              <span className="text-xs text-gray-400 font-mono">{commits.length} commits</span>
            </div>
            <div className="divide-y divide-gray-100">
              {commits.map((commit, index) => (
                <div 
                  key={commit.id} 
                  onClick={() => setSelectedCommitIdx(index)}
                  className={`p-3.5 cursor-pointer hover:bg-gray-50 transition-colors ${selectedCommitIdx === index ? 'bg-green-50/50 border-l-4 border-l-green-600' : 'border-l-4 border-l-transparent'}`}
                >
                  <div className="flex items-start space-x-2.5">
                    <div className="mt-0.5 text-gray-400">
                      <GitCommit className="w-4 h-4" />
                    </div>
                    <div className="flex-1 min-w-0">
                      <p className="text-sm font-medium text-gray-900 truncate">{commit.message}</p>
                      <div className="flex items-center mt-1 text-xs text-gray-500 space-x-2">
                        <span className="font-mono bg-gray-100 px-1 py-0.2 rounded text-[11px]">{commit.hash}</span>
                        <span>•</span>
                        <span>{new Date(commit.timestamp).toLocaleString()}</span>
                      </div>
                    </div>
                  </div>
                </div>
              ))}
            </div>
          </div>
          
          {/* Right: Key-Value Pairs */}
          <div className="w-full md:w-2/3 bg-white flex flex-col flex-1 h-full overflow-hidden">
            {commits.length > 0 && commits[selectedCommitIdx] ? (
              <>
                <div className="px-4 md:px-6 py-3 border-b border-gray-200 flex justify-between items-center bg-gray-50/50 sticky top-0 z-10">
                  <div>
                    <h3 className="text-sm font-bold text-gray-900">{commits[selectedCommitIdx].message}</h3>
                    <p className="text-xs text-gray-500 mt-0.5 font-mono">commit {commits[selectedCommitIdx].hash} ({selectedBranch})</p>
                  </div>
                  <Button variant="outline" size="sm" onClick={handleRevertCommit} className="text-xs h-8 border-gray-300 text-gray-700 hover:text-black">
                    Revert Commit
                  </Button>
                </div>

                <div className="p-4 md:p-6 overflow-y-auto flex-1">
                  <div className="space-y-3">
                    {secrets.map((secret) => {
                      const isVisible = !!revealed[secret.key_name];
                      return (
                        <div key={secret.key_name} className="flex flex-col sm:flex-row gap-2 items-stretch sm:items-center">
                          <div className="w-full sm:w-1/3">
                            <div className="flex items-center space-x-2 border border-gray-200 bg-gray-50 px-3 py-2 text-xs font-mono text-gray-800 font-semibold truncate">
                              <Key className="w-3.5 h-3.5 text-gray-500 shrink-0" />
                              <span className="truncate">{secret.key_name}</span>
                            </div>
                      </div>
                      <div className="w-full sm:w-2/3 flex items-center gap-1.5">
                        <div className="relative flex-1">
                          <input 
                            type={isVisible ? "text" : "password"} 
                            value={secret.encrypted_value} 
                            readOnly 
                            className="w-full border border-gray-200 px-3 py-2 text-xs font-mono text-gray-900 bg-white focus:outline-none" 
                          />
                        </div>
                        <Button
                          variant="ghost"
                          size="sm"
                          onClick={() => toggleReveal(secret.key_name)}
                          className="h-8 px-2 text-gray-500 hover:text-gray-900"
                          title={isVisible ? "Hide secret" : "Reveal secret"}
                        >
                          {isVisible ? <EyeOff className="w-4 h-4" /> : <Eye className="w-4 h-4" />}
                        </Button>
                      </div>
                    </div>
                  );
                })}
              </div>
              
              <div className="mt-8 pt-4 border-t border-gray-100 flex items-center justify-between text-xs text-gray-500">
                <div className="flex items-center">
                  <Shield className="w-4 h-4 mr-1.5 text-green-600" />
                  Values are encrypted at rest with Zero-Knowledge principles.
                </div>
                <button 
                  onClick={() => setShowDownloadModal(true)}
                  className="text-green-700 hover:underline font-medium flex items-center gap-1"
                >
                  Configure via zvault CLI
                  <ExternalLink className="w-3 h-3" />
                </button>
              </div>
            </div>
            </>
            ) : (
              <div className="flex-1 flex items-center justify-center text-gray-500 text-sm">
                No commits found in this repository.
              </div>
            )}
          </div>
        </div>
      </div>

      {/* Download CLI Modal */}
      {showDownloadModal && (
        <div className="fixed inset-0 z-50 flex items-center justify-center bg-black/60 backdrop-blur-sm p-4 animate-in fade-in duration-200">
          <div className="bg-white rounded-none border border-gray-300 shadow-2xl max-w-2xl w-full max-h-[90vh] overflow-y-auto flex flex-col">
            {/* Modal Header */}
            <div className="flex items-center justify-between px-6 py-4 border-b border-gray-200 bg-gray-50">
              <div className="flex items-center space-x-2.5">
                <div className="p-1.5 bg-green-100 text-green-800 rounded">
                  <Terminal className="w-5 h-5" />
                </div>
                <div>
                  <h3 className="font-bold text-base text-gray-900">Download zvault CLI</h3>
                  <p className="text-xs text-gray-500">Official cross-platform Git-style secrets management tool</p>
                </div>
              </div>
              <button 
                onClick={() => setShowDownloadModal(false)}
                className="text-gray-400 hover:text-gray-600 p-1"
              >
                <X className="w-5 h-5" />
              </button>
            </div>

            {/* Modal Content */}
            <div className="p-6 space-y-6">
              {/* OS & Arch Selector */}
              <div>
                <h4 className="text-xs font-bold uppercase tracking-wider text-gray-600 mb-3">Pilih Sistem Operasi Anda</h4>
                <div className="flex flex-wrap gap-2 mb-4">
                  {downloadOptions.map((opt) => {
                    const Icon = opt.icon;
                    return (
                      <button
                        key={opt.os}
                        onClick={() => { setDlOS(opt.os); setDlArch(0); }}
                        className={`flex items-center gap-2 px-4 py-2 border rounded text-sm transition-colors ${dlOS === opt.os ? 'border-green-600 bg-green-50 text-green-800 font-semibold' : 'border-gray-200 hover:bg-gray-50 text-gray-700'}`}
                      >
                        <Icon className="w-4 h-4" />
                        {opt.os}
                      </button>
                    );
                  })}
                </div>
                
                {downloadOptions.find(o => o.os === dlOS) && (
                  <div className="space-y-4">
                    <div>
                      <label className="text-xs font-bold uppercase tracking-wider text-gray-600 mb-2 block">Arsitektur</label>
                      <select 
                        value={dlArch} 
                        onChange={(e) => setDlArch(Number(e.target.value))}
                        className="w-full sm:w-auto bg-white border border-gray-300 text-gray-700 text-sm py-2 px-3 rounded focus:outline-none focus:ring-1 focus:ring-green-500"
                      >
                        {downloadOptions.find(o => o.os === dlOS)!.archs.map((a, i) => (
                          <option key={i} value={i}>{a.label}</option>
                        ))}
                      </select>
                    </div>

                    <div>
                      <label className="text-xs font-bold uppercase tracking-wider text-gray-600 mb-2 block">Terminal Command</label>
                      <div className="relative bg-gray-900 text-gray-100 p-3.5 rounded font-mono text-xs flex flex-col sm:flex-row sm:items-center justify-between gap-3 overflow-hidden">
                        <code className="break-all">
                          {downloadOptions.find(o => o.os === dlOS)!.getInstallCmd(downloadOptions.find(o => o.os === dlOS)!.archs[dlArch].filename)}
                        </code>
                        <button 
                          onClick={() => handleCopy(downloadOptions.find(o => o.os === dlOS)!.getInstallCmd(downloadOptions.find(o => o.os === dlOS)!.archs[dlArch].filename), 'quick-install')}
                          className="text-gray-400 hover:text-white shrink-0 self-end sm:self-auto flex items-center gap-1.5 bg-gray-800 px-2 py-1 rounded"
                          title="Copy command"
                        >
                          {copiedCmd === 'quick-install' ? (
                            <><Check className="w-3.5 h-3.5 text-green-400" /> Copied</>
                          ) : (
                            <><Copy className="w-3.5 h-3.5" /> Copy</>
                          )}
                        </button>
                      </div>
                      <p className="text-[11px] text-gray-500 mt-2">
                        Command di atas tidak memerlukan akses root/sudo dan akan memasang binary ke <code>~/.local/bin/</code>. Pastikan direktori tersebut ada di <code>PATH</code> Anda.
                      </p>
                    </div>

                    <div className="pt-2 flex flex-col sm:flex-row gap-3 items-center justify-between border-t border-gray-100">
                      <a 
                        href={downloadOptions.find(o => o.os === dlOS)!.archs[dlArch].url}
                        download={downloadOptions.find(o => o.os === dlOS)!.archs[dlArch].filename}
                        className="flex items-center gap-2 text-sm font-semibold text-green-700 hover:text-green-800 bg-green-50 px-4 py-2 rounded"
                      >
                        <Download className="w-4 h-4" />
                        Unduh Manual ({downloadOptions.find(o => o.os === dlOS)!.archs[dlArch].filename})
                      </a>
                      
                      <div className="flex items-center gap-1.5 text-[11px] text-gray-500">
                        <ExternalLink className="w-3 h-3" />
                        <a href="https://github.com/kelvinzer0/zcdns/releases" target="_blank" rel="noopener noreferrer" className="hover:underline">
                          Lihat Source / GitHub Releases
                        </a>
                      </div>
                    </div>
                  </div>
                )}
              </div>

              {/* Basic Usage Cheat Sheet */}
              <div>
                <h4 className="text-xs font-bold uppercase tracking-wider text-gray-600 mb-2">Quick Start Commands</h4>
                <div className="bg-gray-900 text-gray-100 p-3.5 font-mono text-xs space-y-1.5 overflow-x-auto">
                  <div className="text-gray-400"># 1. Login to your account</div>
                  <div className="text-green-400">zvault login</div>
                  <div className="text-gray-400 mt-2"># 2. Clone your vault repo</div>
                  <div className="text-green-400">zvault clone https://zcdns.id/vault/your-repo</div>
                  <div className="text-gray-400 mt-2"># 3. Add or update secrets</div>
                  <div className="text-green-400">zvault set API_KEY=sk_live_...</div>
                  <div className="text-gray-400 mt-2"># 4. Commit and push</div>
                  <div className="text-green-400">zvault commit -m &quot;update production API key&quot;</div>
                  <div className="text-green-400">zvault push</div>
                </div>
              </div>
            </div>

            {/* Modal Footer */}
            <div className="px-6 py-3 border-t border-gray-200 bg-gray-50 flex justify-end">
              <Button 
                variant="outline"
                size="sm"
                onClick={() => setShowDownloadModal(false)}
                className="text-xs"
              >
                Close
              </Button>
            </div>
          </div>
        </div>
      )}
    </div>
  );
}
