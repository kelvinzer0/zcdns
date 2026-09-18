import { useState } from 'react';
import { Copy, Check, RefreshCw, Terminal, Globe, LogOut, ShieldCheck, Clock, AlertTriangle } from 'lucide-react';
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
  const [showConfirmNew, setShowConfirmNew] = useState(false);

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
            onClick={() => setShowConfirmNew(true)}
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
      <div className="mt-6 pt-5 border-t border-gray-100">
        <div className="flex flex-col sm:flex-row sm:items-center justify-between gap-3">
          <span className="text-xs font-semibold text-gray-500 uppercase tracking-wider flex items-center shrink-0">
            <Terminal className="w-3.5 h-3.5 mr-1.5" />
            Uji Resolusi DNS
          </span>
          <div className="bg-[#0D1117] border border-gray-800 flex-1 flex items-center justify-between overflow-hidden relative group w-full max-w-full sm:max-w-md ml-auto">
            <div className="flex items-center text-[11px] font-mono text-gray-300 min-w-0 overflow-x-auto pl-3 pr-2 py-2 hide-scrollbar">
              <span className="text-green-400 mr-2 select-none">$</span>
              <span className="whitespace-nowrap select-all">{digCmd}</span>
            </div>
            <button
              onClick={() => copyToClipboard(digCmd, setCopiedDig)}
              className="p-2 bg-[#0D1117]/80 backdrop-blur-sm text-gray-400 hover:text-white transition-colors shrink-0 absolute right-0 border-l border-gray-800"
              title={t('btn-copy')}
            >
              {copiedDig ? <Check className="w-3.5 h-3.5 text-green-400" /> : <Copy className="w-3.5 h-3.5" />}
            </button>
          </div>
        </div>
      </div>

      {/* Confirmation Modal for New Subdomain */}
      {showConfirmNew && (
        <div className="fixed inset-0 z-50 flex items-center justify-center p-4 bg-black/50 backdrop-blur-xs animate-in fade-in">
          <div className="bg-white border border-gray-300 shadow-xl max-w-md w-full p-6 space-y-4 rounded-none">
            <div className="flex items-start space-x-3">
              <div className="p-2.5 bg-red-50 text-red-600 shrink-0 border border-red-200">
                <AlertTriangle className="w-6 h-6" />
              </div>
              <div className="space-y-1">
                <h3 className="text-base sm:text-lg font-bold text-gray-900">
                  {t('confirm-new-subdomain-title')}
                </h3>
                <p className="text-xs sm:text-sm text-gray-600 leading-relaxed">
                  {t('confirm-new-subdomain')}
                </p>
                <div className="mt-2 p-2 bg-gray-50 border border-gray-200 text-xs font-mono text-gray-700 break-all">
                  Subdomain aktif: <span className="font-bold text-red-600">{fullDomain}</span>
                </div>
              </div>
            </div>

            <div className="flex items-center justify-end space-x-2 pt-3 border-t border-gray-200">
              <Button
                variant="outline"
                size="sm"
                onClick={() => setShowConfirmNew(false)}
                className="rounded-none border-gray-300 text-gray-700"
              >
                {t('confirm-new-subdomain-cancel')}
              </Button>
              <Button
                size="sm"
                onClick={() => {
                  setShowConfirmNew(false);
                  onNewSession();
                }}
                className="bg-red-600 hover:bg-red-700 text-white rounded-none font-semibold"
              >
                {t('confirm-new-subdomain-confirm')}
              </Button>
            </div>
          </div>
        </div>
      )}
    </div>
  );
};
