import { useState, useEffect } from 'react';
import { useSearchParams, Link } from 'react-router-dom';
import { Shield, Terminal, CheckCircle, XCircle, AlertCircle, Loader2, ArrowRight } from 'lucide-react';
import { Button } from './ui/button';

export function VaultAuthPage() {
  const [searchParams] = useSearchParams();
  const codeParam = searchParams.get('code') || '';

  const [code, setCode] = useState(codeParam.toUpperCase());
  const [subdomain, setSubdomain] = useState('');
  const [status, setStatus] = useState<'idle' | 'checking' | 'ready' | 'submitting' | 'approved' | 'denied' | 'error'>('idle');
  const [errorMessage, setErrorMessage] = useState('');

  // Load existing session subdomain from localStorage if available
  useEffect(() => {
    try {
      const savedSession = localStorage.getItem('zcdns_session');
      if (savedSession) {
        const parsed = JSON.parse(savedSession);
        if (parsed?.subdomain) {
          setSubdomain(parsed.subdomain);
        }
      }
    } catch {
      // ignore
    }
  }, []);

  // If code is in URL, automatically check it
  useEffect(() => {
    if (codeParam) {
      verifyCode(codeParam);
    }
  }, [codeParam]);

  const verifyCode = async (userCode: string) => {
    const cleanCode = userCode.trim().toUpperCase();
    if (!cleanCode) return;

    setStatus('checking');
    setErrorMessage('');

    try {
      const res = await fetch(`/api/vault/auth/info?code=${encodeURIComponent(cleanCode)}`);
      const data = await res.json();

      if (!res.ok || data.error) {
        setStatus('error');
        setErrorMessage(data.error || 'Invalid or non-existent verification code');
        return;
      }

      if (data.is_expired) {
        setStatus('error');
        setErrorMessage('This verification code has expired. Please run `zvault login` again.');
        return;
      }

      if (data.status === 'approved') {
        setStatus('approved');
        return;
      }

      if (data.status === 'denied') {
        setStatus('denied');
        return;
      }

      setStatus('ready');
    } catch (err: any) {
      setStatus('error');
      setErrorMessage(err.message || 'Network error checking code');
    }
  };

  const handleAction = async (action: 'approve' | 'deny') => {
    const cleanCode = code.trim().toUpperCase();
    if (!cleanCode) {
      setErrorMessage('Verification code is required');
      return;
    }

    if (action === 'approve' && !subdomain.trim()) {
      setErrorMessage('Please specify the subdomain you want to authenticate');
      return;
    }

    setStatus('submitting');
    setErrorMessage('');

    try {
      const res = await fetch('/api/vault/auth/verify', {
        method: 'POST',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify({
          user_code: cleanCode,
          action,
          subdomain: subdomain.trim().toLowerCase(),
        }),
      });

      const data = await res.json();
      if (!res.ok || data.error) {
        setStatus('error');
        setErrorMessage(data.error || 'Failed to complete authorization');
        return;
      }

      if (action === 'approve') {
        setStatus('approved');
      } else {
        setStatus('denied');
      }
    } catch (err: any) {
      setStatus('error');
      setErrorMessage(err.message || 'Failed to submit authorization');
    }
  };

  return (
    <div className="min-h-[80vh] flex items-center justify-center py-12 px-4 sm:px-6 lg:px-8">
      <div className="max-w-md w-full space-y-6 bg-white border border-gray-200 p-8 shadow-sm">
        {/* Header */}
        <div className="text-center space-y-2">
          <div className="inline-flex p-3 bg-green-50 border border-green-200 text-green-700 rounded-full mb-2">
            <Shield className="w-8 h-8" />
          </div>
          <h1 className="text-xl font-bold text-gray-900">Authorize zvault CLI</h1>
          <p className="text-xs text-gray-500">
            A terminal session is requesting access to your ZCDNS Secrets Vault.
          </p>
        </div>

        {/* State: Approved */}
        {status === 'approved' && (
          <div className="bg-green-50 border border-green-200 p-6 text-center space-y-4">
            <CheckCircle className="w-12 h-12 text-green-600 mx-auto" />
            <div>
              <h3 className="text-base font-bold text-green-900">Successfully Authorized!</h3>
              <p className="text-xs text-green-800 mt-1">
                You can now return to your terminal. Your <code className="font-mono bg-green-100 px-1 py-0.5 rounded">zvault</code> CLI has received the access token for <span className="font-semibold">{subdomain}.zcdns.id</span>.
              </p>
            </div>
            <div className="pt-2">
              <Link to="/dashboard">
                <Button className="w-full bg-green-600 hover:bg-green-500 text-xs">
                  Go to Dashboard
                  <ArrowRight className="w-4 h-4 ml-1.5" />
                </Button>
              </Link>
            </div>
          </div>
        )}

        {/* State: Denied */}
        {status === 'denied' && (
          <div className="bg-gray-50 border border-gray-200 p-6 text-center space-y-4">
            <XCircle className="w-12 h-12 text-red-500 mx-auto" />
            <div>
              <h3 className="text-base font-bold text-gray-900">Authorization Denied</h3>
              <p className="text-xs text-gray-600 mt-1">
                The terminal session request has been declined. You can close this window.
              </p>
            </div>
            <Link to="/">
              <Button variant="outline" className="w-full text-xs">
                Back to Home
              </Button>
            </Link>
          </div>
        )}

        {/* State: Input / Verification */}
        {status !== 'approved' && status !== 'denied' && (
          <div className="space-y-4">
            {errorMessage && (
              <div className="p-3 bg-red-50 border border-red-200 text-red-700 text-xs flex items-center space-x-2">
                <AlertCircle className="w-4 h-4 shrink-0" />
                <span>{errorMessage}</span>
              </div>
            )}

            {/* Code Field */}
            <div>
              <label className="block text-xs font-semibold uppercase tracking-wider text-gray-700 mb-1">
                One-Time Device Code
              </label>
              <div className="relative">
                <input
                  type="text"
                  value={code}
                  onChange={(e) => {
                    const val = e.target.value.toUpperCase();
                    setCode(val);
                    if (val.length >= 8) {
                      verifyCode(val);
                    }
                  }}
                  placeholder="e.g. WDJB-MJHT"
                  className="w-full border border-gray-300 px-3 py-2.5 text-center font-mono text-lg font-bold tracking-widest text-gray-900 uppercase focus:outline-none focus:ring-1 focus:ring-green-600"
                />
                {status === 'checking' && (
                  <div className="absolute right-3 top-1/2 -translate-y-1/2 text-gray-400">
                    <Loader2 className="w-4 h-4 animate-spin" />
                  </div>
                )}
              </div>
            </div>

            {/* Subdomain profile display */}
            <div>
              <label className="block text-xs font-semibold uppercase tracking-wider text-gray-700 mb-1">
                Granting Access For
              </label>
              {subdomain ? (
                <div className="flex items-center">
                  <div className="flex-1 bg-gray-50 border border-gray-300 px-3 py-2 text-sm font-mono text-gray-700">
                    {subdomain}
                  </div>
                  <span className="bg-gray-100 border border-l-0 border-gray-300 px-3 py-2 text-sm font-mono text-gray-500">
                    .zcdns.id
                  </span>
                </div>
              ) : (
                <div className="p-3 bg-amber-50 border border-amber-200 text-amber-700 text-xs">
                  <span className="font-semibold">Sesi tidak ditemukan!</span> Anda harus <Link to="/" className="underline font-bold hover:text-amber-800">login</Link> terlebih dahulu untuk memberi izin akses.
                </div>
              )}
              <p className="text-[11px] text-gray-500 mt-1">
                The zvault CLI will be permanently bound to this specific vault repository.
              </p>
            </div>

            {/* Info notice */}
            <div className="p-3 bg-gray-50 border border-gray-200 text-xs text-gray-600 space-y-1">
              <div className="flex items-center space-x-1.5 font-medium text-gray-800">
                <Terminal className="w-3.5 h-3.5 text-green-700" />
                <span>Command Line Authorization</span>
              </div>
              <p className="text-[11px]">
                Approving this request allows zvault CLI to pull and push encrypted secrets directly from your terminal.
              </p>
            </div>

            {/* Action Buttons */}
            <div className="flex items-center gap-3 pt-2">
              <Button
                onClick={() => handleAction('approve')}
                disabled={status === 'submitting' || !code.trim() || !subdomain.trim()}
                className="flex-1 bg-green-600 hover:bg-green-500 text-white text-xs font-semibold py-2.5 rounded-none"
              >
                {status === 'submitting' ? (
                  <Loader2 className="w-4 h-4 animate-spin mx-auto" />
                ) : (
                  'Approve Access'
                )}
              </Button>
              <Button
                variant="outline"
                onClick={() => handleAction('deny')}
                disabled={status === 'submitting' || !code.trim()}
                className="flex-1 text-xs border-gray-300 text-gray-700 hover:text-black py-2.5 rounded-none"
              >
                Deny
              </Button>
            </div>
          </div>
        )}
      </div>
    </div>
  );
}
