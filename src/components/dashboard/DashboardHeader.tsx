import { useState } from 'react';
import { Copy, Check, RefreshCw, Terminal, Globe, LogOut } from 'lucide-react';
import { Button } from '../ui/button';
import type { UserSession } from './types';

interface Props {
  session: UserSession;
  wsStatus: 'connected' | 'disconnected' | 'connecting';
  onNewSession: () => void;
  onLogout: () => void;
}

export const DashboardHeader: React.FC<Props> = ({
  session,
  wsStatus,
  onNewSession,
  onLogout,
}) => {
  const [copiedDomain, setCopiedDomain] = useState(false);
  const [copiedDig, setCopiedDig] = useState(false);

  const fullDomain = session.domain || `${session.subdomain}.${session.baseDomain || 'zcdns.id'}`;
  const nameserver = `ns1.${session.baseDomain || 'zcdns.id'}`;
  const digCmd = session.dnsPort && session.dnsPort !== 53
    ? `dig @${nameserver} -p ${session.dnsPort} ${fullDomain} ANY`
    : `dig @${nameserver} ${fullDomain} ANY`;

  const copyToClipboard = (text: string, setCopied: (v: boolean) => void) => {
    navigator.clipboard.writeText(text);
    setCopied(true);
    setTimeout(() => setCopied(false), 2000);
  };

  return (
    <div className="bg-white rounded-2xl border border-gray-200 shadow-sm p-6 mb-8">
      <div className="flex flex-col lg:flex-row lg:items-center lg:justify-between gap-6">
        {/* Subdomain Info */}
        <div className="space-y-2">
          <div className="flex items-center space-x-2">
            <span className="text-xs font-semibold uppercase tracking-wider text-green-700 bg-green-50 px-2.5 py-1 rounded-full border border-green-200">
              Active Delegated Sandbox
            </span>
            <div className="flex items-center space-x-1.5 ml-2">
              <span
                className={`w-2.5 h-2.5 rounded-full ${
                  wsStatus === 'connected'
                    ? 'bg-green-500 animate-pulse'
                    : wsStatus === 'connecting'
                    ? 'bg-amber-400 animate-ping'
                    : 'bg-red-400'
                }`}
              />
              <span className="text-xs text-gray-500 font-medium">
                {wsStatus === 'connected'
                  ? 'Real-Time Stream Active'
                  : wsStatus === 'connecting'
                  ? 'Connecting...'
                  : 'Offline'}
              </span>
            </div>
          </div>

          <div className="flex items-center flex-wrap gap-3">
            <h1 className="text-2xl sm:text-3xl font-bold text-gray-900 flex items-center font-mono">
              <Globe className="w-6 h-6 mr-2.5 text-green-600 inline" />
              {fullDomain}
            </h1>
            <button
              onClick={() => copyToClipboard(fullDomain, setCopiedDomain)}
              className="inline-flex items-center space-x-1 px-3 py-1.5 text-xs font-medium text-gray-700 bg-gray-100 hover:bg-gray-200 rounded-lg transition-colors border border-gray-200"
              title="Copy subdomain"
            >
              {copiedDomain ? (
                <>
                  <Check className="w-3.5 h-3.5 text-green-600" />
                  <span className="text-green-700">Copied!</span>
                </>
              ) : (
                <>
                  <Copy className="w-3.5 h-3.5 text-gray-500" />
                  <span>Copy Domain</span>
                </>
              )}
            </button>
          </div>
          <p className="text-sm text-gray-600 max-w-2xl">
            You have full authoritative DNS control over this subdomain. Any queries sent to this domain will resolve according to your records and appear in the live requests feed below.
          </p>
        </div>

        {/* Actions */}
        <div className="flex flex-wrap items-center gap-3">
          <Button
            variant="outline"
            size="sm"
            onClick={onNewSession}
            className="text-gray-700 hover:text-green-600 hover:border-green-300"
          >
            <RefreshCw className="w-4 h-4 mr-2" />
            New Subdomain
          </Button>
          <Button
            variant="ghost"
            size="sm"
            onClick={onLogout}
            className="text-gray-500 hover:text-red-600"
          >
            <LogOut className="w-4 h-4 mr-1.5" />
            Exit
          </Button>
        </div>
      </div>

      {/* Terminal Quick Hint */}
      <div className="mt-5 pt-4 border-t border-gray-100 flex flex-col sm:flex-row sm:items-center justify-between gap-3 text-xs bg-gray-50/80 -mx-6 -mb-6 p-4 rounded-b-2xl">
        <div className="flex items-center space-x-2 text-gray-700 font-mono">
          <Terminal className="w-4 h-4 text-gray-500 flex-shrink-0" />
          <span className="text-gray-500">Query your domain from terminal:</span>
          <code className="bg-white px-2 py-1 rounded border border-gray-200 text-gray-900 select-all font-semibold">
            {digCmd}
          </code>
        </div>
        <button
          onClick={() => copyToClipboard(digCmd, setCopiedDig)}
          className="text-green-700 hover:text-green-800 font-medium flex items-center space-x-1 self-end sm:self-auto cursor-pointer"
        >
          {copiedDig ? (
            <>
              <Check className="w-3.5 h-3.5 text-green-600" />
              <span>Copied Command</span>
            </>
          ) : (
            <>
              <Copy className="w-3.5 h-3.5" />
              <span>Copy dig command</span>
            </>
          )}
        </button>
      </div>
    </div>
  );
};
