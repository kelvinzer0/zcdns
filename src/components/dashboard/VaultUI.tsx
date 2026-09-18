import { useState } from 'react';
import { GitCommit, Key, Shield, ChevronDown } from 'lucide-react';
import { Button } from '../ui/button';

export function VaultUI() {
  const [selectedBranch, setSelectedBranch] = useState('main');
  const [selectedCommit, setSelectedCommit] = useState(0);

  const branches = ['main', 'production', 'staging'];
  
  const commits = [
    { id: 0, hash: 'a1b2c3d', message: 'update production API key', author: 'user', time: '2 mins ago' },
    { id: 1, hash: 'e4f5g6h', message: 'add database credentials', author: 'system', time: '1 hr ago' },
    { id: 2, hash: 'i7j8k9l', message: 'initial secrets', author: 'user', time: '2 days ago' },
  ];

  const secrets = [
    { key: 'API_KEY', value: 'sk_live_1234567890abcdef' },
    { key: 'DB_PASSWORD', value: 'super_secret_password' },
    { key: 'JWT_SECRET', value: 'my_jwt_secret_key' },
  ];

  return (
    <div className="bg-white rounded-none border border-gray-200 shadow-sm overflow-hidden mt-6">
      <div className="border-b border-gray-200 bg-gray-50 px-4 py-3 flex items-center justify-between">
        <div className="flex items-center space-x-2">
          <Shield className="w-5 h-5 text-gray-700" />
          <h2 className="text-lg font-bold text-gray-900">Secrets Vault</h2>
        </div>
        <div className="flex items-center space-x-2">
          <span className="text-sm text-gray-500 font-medium">Branch:</span>
          <div className="relative">
            <select
              value={selectedBranch}
              onChange={(e) => setSelectedBranch(e.target.value)}
              className="appearance-none bg-white border border-gray-300 text-gray-700 text-sm py-1.5 pl-3 pr-8 rounded-md focus:outline-none focus:ring-1 focus:ring-green-500 focus:border-green-500 font-medium cursor-pointer"
            >
              {branches.map(b => <option key={b} value={b}>{b}</option>)}
            </select>
            <ChevronDown className="w-4 h-4 text-gray-500 absolute right-2 top-1/2 transform -translate-y-1/2 pointer-events-none" />
          </div>
        </div>
      </div>
      
      <div className="flex flex-col md:flex-row h-[600px]">
        {/* Left: Commit History */}
        <div className="w-full md:w-1/3 border-r border-gray-200 bg-white overflow-y-auto">
          <div className="px-4 py-3 border-b border-gray-200 bg-gray-50 sticky top-0">
            <h3 className="text-sm font-semibold text-gray-700">Commit History</h3>
          </div>
          <div className="divide-y divide-gray-100">
            {commits.map((commit, index) => (
              <div 
                key={commit.id} 
                onClick={() => setSelectedCommit(index)}
                className={`p-4 cursor-pointer hover:bg-gray-50 transition-colors ${selectedCommit === index ? 'bg-blue-50/50 border-l-4 border-l-blue-500' : 'border-l-4 border-l-transparent'}`}
              >
                <div className="flex items-start space-x-3">
                  <div className="mt-0.5 text-gray-400">
                    <GitCommit className="w-4 h-4" />
                  </div>
                  <div className="flex-1 min-w-0">
                    <p className="text-sm font-medium text-gray-900 truncate">{commit.message}</p>
                    <div className="flex items-center mt-1 text-xs text-gray-500 space-x-2">
                      <span className="font-mono">{commit.hash}</span>
                      <span>•</span>
                      <span>{commit.time}</span>
                    </div>
                  </div>
                </div>
              </div>
            ))}
          </div>
        </div>
        
        {/* Right: Key-Value Pairs */}
        <div className="w-full md:w-2/3 bg-white flex flex-col h-full overflow-hidden">
          <div className="px-6 py-4 border-b border-gray-200 flex justify-between items-center bg-white sticky top-0 z-10">
            <div>
              <h3 className="text-lg font-bold text-gray-900">{commits[selectedCommit].message}</h3>
              <p className="text-sm text-gray-500 mt-1 font-mono">commit {commits[selectedCommit].hash}</p>
            </div>
            <Button className="bg-gray-900 text-white hover:bg-gray-800 text-sm py-1.5 px-4 h-auto">
              Revert
            </Button>
          </div>
          <div className="p-6 overflow-y-auto flex-1">
            <div className="space-y-4">
              {secrets.map((secret, i) => (
                <div key={i} className="flex flex-col sm:flex-row gap-3">
                  <div className="w-full sm:w-1/3">
                    <div className="flex items-center space-x-2 border border-gray-300 rounded-md bg-gray-50 px-3 py-2">
                      <Key className="w-4 h-4 text-gray-400" />
                      <span className="text-sm font-mono text-gray-700 truncate">{secret.key}</span>
                    </div>
                  </div>
                  <div className="w-full sm:w-2/3">
                    <div className="relative">
                      <input 
                        type="password" 
                        value={secret.value} 
                        readOnly 
                        className="w-full border border-gray-300 rounded-md px-3 py-2 text-sm font-mono text-gray-900 bg-white tracking-widest focus:outline-none" 
                      />
                    </div>
                  </div>
                </div>
              ))}
            </div>
            
            <div className="mt-8 pt-6 border-t border-gray-100 flex items-center justify-center text-sm text-gray-500">
              <Shield className="w-4 h-4 mr-2" />
              Values are securely encrypted at rest.
            </div>
          </div>
        </div>
      </div>
    </div>
  );
}
