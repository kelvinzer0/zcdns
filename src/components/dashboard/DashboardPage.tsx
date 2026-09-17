import { useState, useEffect, useCallback } from 'react';
import { Sparkles, Database, Radio, Terminal, ArrowRight, Zap, Shield } from 'lucide-react';
import { Button } from '../ui/button';
import { DashboardHeader } from './DashboardHeader';
import { RecordManager } from './RecordManager';
import { LiveRequests } from './LiveRequests';
import { DnsTester } from './DnsTester';
import { Experiments } from './Experiments';
import { ParentalControl } from './ParentalControl';
import type { DnsRecord, DnsRequestLog, Experiment, RecordType, UserSession } from './types';
import { dashboardApi } from './api';

export function DashboardPage() {
  const [session, setSession] = useState<UserSession | null>(null);
  const [isLoadingSession, setIsLoadingSession] = useState(true);

  const [records, setRecords] = useState<DnsRecord[]>([]);
  const [isLoadingRecords, setIsLoadingRecords] = useState(false);

  const [requests, setRequests] = useState<DnsRequestLog[]>([]);
  const [wsStatus, setWsStatus] = useState<'connected' | 'disconnected' | 'connecting'>('disconnected');

  const [activeTab, setActiveTab] = useState<'records' | 'stream' | 'tester' | 'experiments' | 'parental'>('records');

  // Load session on mount
  const loadSession = useCallback(async () => {
    setIsLoadingSession(true);
    try {
      const sess = await dashboardApi.getSession();
      setSession(sess);
      if (sess.logged_in && sess.subdomain) {
        loadRecords(sess.subdomain);
        loadRequests(sess.subdomain);
      }
    } catch {
      setSession({ logged_in: false });
    } finally {
      setIsLoadingSession(false);
    }
  }, []);

  const loadRecords = async (subdomain: string) => {
    setIsLoadingRecords(true);
    try {
      const data = await dashboardApi.getRecords(subdomain);
      setRecords(data);
    } catch (err) {
      console.error('Failed to load records:', err);
    } finally {
      setIsLoadingRecords(false);
    }
  };

  const loadRequests = async (subdomain: string) => {
    try {
      const data = await dashboardApi.getRequests(subdomain);
      setRequests(data);
    } catch (err) {
      console.error('Failed to load requests:', err);
    }
  };

  useEffect(() => {
    loadSession();
  }, [loadSession]);

  // Connect WebSocket stream when session is active
  useEffect(() => {
    if (!session?.logged_in || !session?.subdomain) return;

    const cleanupWs = dashboardApi.connectWebSocket(
      session.subdomain,
      (newLog) => {
        setRequests((prev) => [newLog, ...prev.slice(0, 99)]);
      },
      (status) => {
        setWsStatus(status);
      }
    );

    return () => {
      cleanupWs();
    };
  }, [session?.logged_in, session?.subdomain]);

  const handleStartSession = async () => {
    setIsLoadingSession(true);
    try {
      const newSession = await dashboardApi.createSession();
      setSession(newSession);
      if (newSession.subdomain) {
        await loadRecords(newSession.subdomain);
        await loadRequests(newSession.subdomain);
      }
    } catch (err: any) {
      alert(err.message || 'Failed to start sandbox session');
    } finally {
      setIsLoadingSession(false);
    }
  };

  const handleLogout = async () => {
    await dashboardApi.deleteSession();
    setSession({ logged_in: false });
    setRecords([]);
    setRequests([]);
  };

  const handleAddRecord = async (record: { name: string; type: RecordType; value: string; ttl: number }) => {
    if (!session?.subdomain) return;
    const newRecord = await dashboardApi.createRecord(session.subdomain, record);
    setRecords((prev) => [...prev, newRecord]);
  };

  const handleUpdateRecord = async (id: string, record: { name: string; type: RecordType; value: string; ttl: number }) => {
    if (!session?.subdomain) return;
    const updated = await dashboardApi.updateRecord(session.subdomain, id, record);
    setRecords((prev) => prev.map((r) => (r.id === id ? updated : r)));
  };

  const handleDeleteRecord = async (id: string) => {
    if (!session?.subdomain) return;
    await dashboardApi.deleteRecord(session.subdomain, id);
    setRecords((prev) => prev.filter((r) => r.id !== id));
  };

  const handleClearAllRecords = async () => {
    if (!session?.subdomain) return;
    if (!confirm('Are you sure you want to delete all DNS records in this sandbox?')) return;
    await dashboardApi.deleteAllRecords(session.subdomain);
    setRecords([]);
  };

  const handleClearRequests = async () => {
    if (!session?.subdomain) return;
    await dashboardApi.deleteRequests(session.subdomain);
    setRequests([]);
  };

  const handleSelectExperiment = (exp: Experiment) => {
    if (exp.suggestedRecord) {
      setActiveTab('records');
    } else if (exp.suggestedQuery) {
      setActiveTab('tester');
    }
  };

  if (isLoadingSession) {
    return (
      <div className="min-h-screen pt-32 pb-20 flex items-center justify-center bg-gray-50">
        <div className="text-center space-y-3">
          <div className="w-10 h-10 border-4 border-green-600 border-t-transparent rounded-full animate-spin mx-auto" />
          <p className="text-sm font-medium text-gray-600">Connecting to ZeroCentDNS Engine...</p>
        </div>
      </div>
    );
  }

  // If not logged in / no sandbox session
  if (!session?.logged_in) {
    return (
      <div className="min-h-screen pt-32 pb-20 px-4 sm:px-6 lg:px-8 bg-gradient-to-b from-gray-50 via-white to-gray-50">
        <div className="container mx-auto max-w-4xl">
          {/* Welcome Banner */}
          <div className="text-center space-y-6 mb-16">
            <div className="inline-flex items-center space-x-2 bg-green-50 border border-green-200 text-green-700 px-3 py-1 rounded-none text-xs font-semibold">
              <Zap className="w-3.5 h-3.5 text-green-600" />
              <span>DNS Sandbox Playground</span>
            </div>

            <h1 className="text-3xl sm:text-4xl font-extrabold text-gray-900 tracking-tight leading-tight">
              Eksperimen DNS Langsung di <span className="text-green-600">ZeroCentDNS</span>
            </h1>

            <p className="text-base text-gray-600 max-w-2xl mx-auto leading-relaxed">
              Subdomain terisolasi dengan authoritative DNS server live. Konfigurasi record, uji resolusi, dan pantau kueri secara real-time.
            </p>

            <div className="pt-4">
              <Button
                size="lg"
                onClick={handleStartSession}
                className="bg-[#012241] hover:bg-[#02365f] text-white px-6 py-5 sm:px-8 sm:py-6 text-base sm:text-lg font-bold shadow-md hover:shadow-lg transition-all rounded-none cursor-pointer"
              >
                <span>Mulai Eksperimen DNS</span>
                <ArrowRight className="w-5 h-5 ml-2.5" />
              </Button>
              <p className="text-xs text-gray-400 mt-2.5">
                Tanpa registrasi kartu kredit • Langsung dialokasikan subdomain aktif
              </p>
            </div>
          </div>

          {/* Feature Pillars */}
          <div className="grid grid-cols-1 md:grid-cols-3 gap-4 sm:gap-6">
            <div className="bg-white p-5 sm:p-6 border border-gray-200 shadow-xs space-y-3">
              <div className="w-9 h-9 bg-blue-100 text-blue-700 flex items-center justify-center font-bold">
                <Database className="w-5 h-5" />
              </div>
              <h3 className="font-bold text-gray-900 text-base">DNS Authoritative Penuh</h3>
              <p className="text-xs sm:text-sm text-gray-500 leading-relaxed">
                Kelola record A, AAAA, CNAME, TXT, MX, NS, PTR, CAA, dan SRV dengan TTL fleksibel dan propagasi instan.
              </p>
            </div>

            <div className="bg-white p-5 sm:p-6 border border-gray-200 shadow-xs space-y-3">
              <div className="w-9 h-9 bg-green-100 text-green-700 flex items-center justify-center font-bold">
                <Radio className="w-5 h-5" />
              </div>
              <h3 className="font-bold text-gray-900 text-base">Live Query Stream</h3>
              <p className="text-xs sm:text-sm text-gray-500 leading-relaxed">
                Setiap kueri DNS ke subdomain Anda disiarkan secara langsung detik itu juga melalui koneksi WebSocket.
              </p>
            </div>

            <div className="bg-white p-5 sm:p-6 border border-gray-200 shadow-xs space-y-3">
              <div className="w-9 h-9 bg-purple-100 text-purple-700 flex items-center justify-center font-bold">
                <Sparkles className="w-5 h-5" />
              </div>
              <h3 className="font-bold text-gray-900 text-base">Lab & Eksperimen Interaktif</h3>
              <p className="text-xs sm:text-sm text-gray-500 leading-relaxed">
                Pelajari cara kerja rantai CNAME, NXDOMAIN caching, verifikasi TXT/SPF, hingga prioritas MX dengan modul terpandu.
              </p>
            </div>
          </div>
        </div>
      </div>
    );
  }

  return (
    <div className="min-h-screen pt-24 sm:pt-28 pb-16 px-3 sm:px-6 lg:px-8 bg-gray-50/60">
      <div className="container mx-auto max-w-6xl">
        {/* Header with Subdomain Info & Status */}
        <DashboardHeader
          session={session}
          wsStatus={wsStatus}
          onNewSession={handleStartSession}
          onLogout={handleLogout}
        />

        {/* Tab Navigation - Optimized for Mobile Scroll */}
        <div className="flex items-center gap-1 border-b border-gray-300 mb-6 overflow-x-auto pb-px scrollbar-none">
          <button
            onClick={() => setActiveTab('records')}
            className={`flex items-center space-x-1.5 py-2.5 px-3 sm:px-4 font-semibold text-xs sm:text-sm border-b-2 transition-all whitespace-nowrap cursor-pointer ${
              activeTab === 'records'
                ? 'border-green-600 text-green-700 bg-white'
                : 'border-transparent text-gray-500 hover:text-gray-800'
            }`}
          >
            <Database className="w-4 h-4" />
            <span>DNS Records</span>
            <span className="text-[11px] bg-gray-100 text-gray-600 px-1.5 py-0.2 font-mono font-bold ml-1">
              {records.length}
            </span>
          </button>

          <button
            onClick={() => setActiveTab('stream')}
            className={`flex items-center space-x-1.5 py-2.5 px-3 sm:px-4 font-semibold text-xs sm:text-sm border-b-2 transition-all whitespace-nowrap cursor-pointer ${
              activeTab === 'stream'
                ? 'border-green-600 text-green-700 bg-white'
                : 'border-transparent text-gray-500 hover:text-gray-800'
            }`}
          >
            <Radio className="w-4 h-4" />
            <span>Live Stream</span>
            {wsStatus === 'connected' && (
              <span className="w-1.5 h-1.5 bg-green-500 animate-pulse ml-1" />
            )}
            <span className="text-[11px] bg-gray-100 text-gray-600 px-1.5 py-0.2 font-mono font-bold ml-1">
              {requests.length}
            </span>
          </button>

          <button
            onClick={() => setActiveTab('tester')}
            className={`flex items-center space-x-1.5 py-2.5 px-3 sm:px-4 font-semibold text-xs sm:text-sm border-b-2 transition-all whitespace-nowrap cursor-pointer ${
              activeTab === 'tester'
                ? 'border-green-600 text-green-700 bg-white'
                : 'border-transparent text-gray-500 hover:text-gray-800'
            }`}
          >
            <Terminal className="w-4 h-4" />
            <span>Web Dig</span>
          </button>

          <button
            onClick={() => setActiveTab('experiments')}
            className={`flex items-center space-x-1.5 py-2.5 px-3 sm:px-4 font-semibold text-xs sm:text-sm border-b-2 transition-all whitespace-nowrap cursor-pointer ${
              activeTab === 'experiments'
                ? 'border-green-600 text-green-700 bg-white'
                : 'border-transparent text-gray-500 hover:text-gray-800'
            }`}
          >
            <Sparkles className="w-4 h-4 text-purple-600" />
            <span>Eksperimen</span>
          </button>

          <button
            onClick={() => setActiveTab('parental')}
            className={`flex items-center space-x-1.5 py-2.5 px-3 sm:px-4 font-semibold text-xs sm:text-sm border-b-2 transition-all whitespace-nowrap cursor-pointer ${
              activeTab === 'parental'
                ? 'border-green-600 text-green-700 bg-white'
                : 'border-transparent text-gray-500 hover:text-gray-800'
            }`}
          >
            <Shield className="w-4 h-4 text-emerald-600" />
            <span>Parental Control</span>
          </button>
        </div>

        {/* Tab Content */}
        <div>
          {activeTab === 'records' && (
            <RecordManager
              subdomain={session.subdomain!}
              baseDomain={session.baseDomain || 'zcdns.id'}
              records={records}
              isLoading={isLoadingRecords}
              onAddRecord={handleAddRecord}
              onUpdateRecord={handleUpdateRecord}
              onDeleteRecord={handleDeleteRecord}
              onClearAll={handleClearAllRecords}
            />
          )}

          {activeTab === 'stream' && (
            <LiveRequests
              requests={requests}
              wsStatus={wsStatus}
              onClearRequests={handleClearRequests}
            />
          )}

          {activeTab === 'tester' && (
            <DnsTester
              subdomain={session.subdomain!}
              baseDomain={session.baseDomain || 'zcdns.id'}
            />
          )}

          {activeTab === 'experiments' && (
            <Experiments
              subdomain={session.subdomain!}
              baseDomain={session.baseDomain || 'zcdns.id'}
              dnsPort={session.dnsPort || 53}
              onSelectExperiment={handleSelectExperiment}
            />
          )}

          {activeTab === 'parental' && (
            <ParentalControl
              subdomain={session.subdomain!}
              baseDomain={session.baseDomain || 'zcdns.id'}
              dnsPort={session.dnsPort || 53}
            />
          )}
        </div>
      </div>
    </div>
  );
}
