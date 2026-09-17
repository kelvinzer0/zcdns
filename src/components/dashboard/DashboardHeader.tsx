import { useState } from 'react';
import { Copy, Check, RefreshCw, Terminal, Globe, LogOut, ShieldCheck, Clock } from 'lucide-react';
import { Button } from '../ui/button';
import type { UserSession } from './types';
import { useTranslations } from '../../lib/useTranslations';

interface Props {
  session: UserSession;
  wsStatus: 'connected' | 'disconnected' | 'connecting';
  onNewSession: () => void;
  onLogout: () => void;
  onRenewSession?: () => Promise<void>;
}

export const DashboardHeader: React.FC<Props> = ({
  session,
  wsStatus,
  onNewSession,
  onLogout,
  onRenewSession,
}) => {
  const t = useTranslations('Dashboard');
  const [copiedDomain, setCopiedDomain] = useState(false);
  const [copiedDig, setCopiedDig] = useState(false);
  const [isRenewing, setIsRenewing] = useState(false);
  const [renewSuccess, setRenewSuccess] = useState(false);

  const handleRenew = async () => {
    if (!onRenewSession || isRenewing) return;
    setIsRenewing(true);
    try {
      await onRenewSession();
      setRenewSuccess(true);
      setTimeout(() => setRenewSuccess(false), 3000);
    } finally {
      setIsRenewing(false);
    }
  };

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
    <div className="bg-white border border-gray-200 shadow-xs p-4 sm:p-6 mb-6">
      <div className="flex flex-col lg:flex-row lg:items-center lg:justify-between gap-4">
        {/* Subdomain Info */}
        <div className="space-y-2">
          <div className="flex items-center space-x-2">
            <span className="text-[11px] font-bold uppercase tracking-wider text-green-700 bg-green-50 px-2 py-0.5 border border-green-200">
              {t('playground-badge')}
            </span>
            <div className="flex items-center space-x-1.5 ml-2">
              <span
                className={`w-2 h-2 ${
                  wsStatus === 'connected'
                    ? 'bg-green-500 animate-pulse'
                    : wsStatus === 'connecting'
                    ? 'bg-amber-400 animate-ping'
                    : 'bg-red-400'
                }`}
              />
              <span className="text-xs text-gray-500 font-mono">
                {wsStatus === 'connected'
                  ? 'Stream Online'
                  : wsStatus === 'connecting'
                  ? 'Connecting...'
                  : 'Offline'}
              </span>
            </div>
          </div>

          <div className="flex items-center flex-wrap gap-2">
            <h1 className="text-xl sm:text-2xl font-bold text-gray-900 flex items-center font-mono break-all">
              <Globe className="w-5 h-5 mr-2 text-green-600 shrink-0" />
              {fullDomain}
            </h1>
            <button
              onClick={() => copyToClipboard(fullDomain, setCopiedDomain)}
              className="inline-flex items-center space-x-1 px-2.5 py-1 text-xs font-medium text-gray-700 bg-gray-50 hover:bg-gray-100 transition-colors border border-gray-300"
              title="Copy subdomain"
            >
              {copiedDomain ? (
                <>
                  <Check className="w-3.5 h-3.5 text-green-600" />
                  <span className="text-green-700">{t('btn-copied')}</span>
                </>
              ) : (
                <>
                  <Copy className="w-3.5 h-3.5 text-gray-500" />
                  <span>{t('btn-copy')}</span>
                </>
              )}
            </button>
          </div>
        </div>

        {/* Actions */}
        <div className="flex items-center flex-wrap gap-2">
          {session.expires_at && (
            <div className="hidden sm:flex items-center space-x-1.5 px-2.5 py-1 bg-amber-50 border border-amber-200 text-amber-800 text-xs font-mono">
              <Clock className="w-3.5 h-3.5 text-amber-600 shrink-0" />
              <span>
                Aktif s/d: {new Date(session.expires_at).toLocaleDateString()}
              </span>
            </div>
          )}

          {onRenewSession && (
            <Button
              variant="outline"
              size="sm"
              onClick={handleRenew}
              disabled={isRenewing}
              className="border-green-600 text-green-700 hover:bg-green-50 rounded-none h-9 text-xs sm:text-sm font-medium"
              title="Perpanjang masa aktif subdomain hingga 6 bulan ke depan"
            >
              <ShieldCheck className="w-3.5 h-3.5 mr-1.5 text-green-600" />
              {renewSuccess ? 'Diperpanjang!' : 'Perpanjang (6 Bln)'}
            </Button>
          )}

          <Button
            variant="outline"
            size="sm"
            onClick={onNewSession}
            className="border-gray-300 text-gray-700 hover:text-green-600 hover:border-green-400 rounded-none h-9 text-xs sm:text-sm"
          >
            <RefreshCw className="w-3.5 h-3.5 mr-1.5" />
            {t('btn-new-subdomain')}
          </Button>
          <Button
            variant="ghost"
            size="sm"
            onClick={onLogout}
            className="text-gray-500 hover:text-red-600 rounded-none h-9 text-xs sm:text-sm"
          >
            <LogOut className="w-3.5 h-3.5 mr-1.5" />
            Logout
          </Button>
        </div>
      </div>

      {/* Terminal Quick Hint */}
      <div className="mt-4 pt-3 border-t border-gray-200 flex flex-col sm:flex-row sm:items-center justify-between gap-2 text-xs bg-gray-50 -mx-4 -mb-4 sm:-mx-6 sm:-mb-6 p-3 sm:p-4">
        <div className="flex items-center space-x-2 text-gray-700 font-mono min-w-0">
          <Terminal className="w-4 h-4 text-gray-500 shrink-0" />
          <code className="bg-white px-2 py-1 border border-gray-300 text-gray-900 select-all font-semibold overflow-x-auto whitespace-nowrap block">
            {digCmd}
          </code>
        </div>
        <button
          onClick={() => copyToClipboard(digCmd, setCopiedDig)}
          className="text-green-700 hover:text-green-800 font-medium flex items-center space-x-1 shrink-0 self-end sm:self-auto cursor-pointer"
        >
          {copiedDig ? (
            <>
              <Check className="w-3.5 h-3.5 text-green-600" />
              <span>{t('btn-copied')}</span>
            </>
          ) : (
            <>
              <Copy className="w-3.5 h-3.5" />
              <span>{t('btn-copy')}</span>
            </>
          )}
        </button>
      </div>
    </div>
  );
};
