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
            <div className="inline-flex items-center space-x-2 bg-green-50 border border-green-200 text-green-700 px-4 py-1.5 rounded-full text-xs font-semibold">
              <Zap className="w-4 h-4 text-green-600" />
              <span>Interactive DNS Sandbox Playground</span>
            </div>

            <h1 className="text-4xl sm:text-5xl font-extrabold text-gray-900 tracking-tight leading-tight">
              Learn DNS by <span className="text-green-600">Experimenting & Breaking Things</span>
            </h1>

            <p className="text-lg text-gray-600 max-w-2xl mx-auto leading-relaxed">
              Inspired by Julia Evans’ Mess-With-DNS, ZeroCentDNS gives you a free, isolated subdomain backed by a live authoritative DNS server. Add records, test resolvers, and watch queries stream in real-time.
            </p>

            <div className="pt-4">
              <Button
                size="lg"
                onClick={handleStartSession}
                className="bg-[#012241] hover:bg-[#02365f] text-white px-8 py-6 text-lg font-bold shadow-xl hover:shadow-2xl transition-all rounded-xl cursor-pointer"
              >
                <span>Start Experimenting Now</span>
                <ArrowRight className="w-5 h-5 ml-2.5" />
              </Button>
              <p className="text-xs text-gray-400 mt-2.5">
                No login or credit card required • Instant ephemeral sandbox allocated
              </p>
            </div>
          </div>

          {/* Feature Pillars */}
          <div className="grid grid-cols-1 md:grid-cols-3 gap-6">
            <div className="bg-white p-6 rounded-2xl border border-gray-200 shadow-sm space-y-3">
              <div className="w-10 h-10 rounded-xl bg-blue-100 text-blue-700 flex items-center justify-center font-bold">
                <Database className="w-5 h-5" />
              </div>
              <h3 className="font-bold text-gray-900 text-base">Full Authoritative Control</h3>
              <p className="text-sm text-gray-500 leading-relaxed">
                Create A, AAAA, CNAME, TXT, MX, NS, PTR, CAA, and SRV records with custom TTLs and immediate propagation.
              </p>
            </div>

            <div className="bg-white p-6 rounded-2xl border border-gray-200 shadow-sm space-y-3">
              <div className="w-10 h-10 rounded-xl bg-green-100 text-green-700 flex items-center justify-center font-bold">
                <Radio className="w-5 h-5" />
              </div>
              <h3 className="font-bold text-gray-900 text-base">Live Query Stream</h3>
              <p className="text-sm text-gray-500 leading-relaxed">
                Every query sent to your subdomain over UDP or TCP is broadcast to your dashboard in real-time via WebSocket.
              </p>
            </div>

            <div className="bg-white p-6 rounded-2xl border border-gray-200 shadow-sm space-y-3">
              <div className="w-10 h-10 rounded-xl bg-purple-100 text-purple-700 flex items-center justify-center font-bold">
                <Sparkles className="w-5 h-5" />
              </div>
              <h3 className="font-bold text-gray-900 text-base">Guided Experiments</h3>
              <p className="text-sm text-gray-500 leading-relaxed">
                Follow step-by-step hands-on DNS labs explaining CNAME chaining, NXDOMAIN caching, SPF verification, and MX priority.
              </p>
            </div>
          </div>
        </div>
      </div>
    );
  }

  return (
    <div className="min-h-screen pt-28 pb-24 px-4 sm:px-6 lg:px-8 bg-gray-50/60">
      <div className="container mx-auto max-w-6xl">
        {/* Header with Subdomain Info & Status */}
        <DashboardHeader
          session={session}
          wsStatus={wsStatus}
          onNewSession={handleStartSession}
          onLogout={handleLogout}
        />

        {/* Tab Navigation */}
        <div className="flex items-center space-x-2 border-b border-gray-200 mb-8 overflow-x-auto pb-px">
          <button
            onClick={() => setActiveTab('records')}
            className={`flex items-center space-x-2 py-3 px-4 font-semibold text-sm border-b-2 transition-all whitespace-nowrap cursor-pointer ${
              activeTab === 'records'
                ? 'border-green-600 text-green-700 bg-white rounded-t-lg shadow-xs'
                : 'border-transparent text-gray-500 hover:text-gray-800'
            }`}
          >
            <Database className="w-4 h-4" />
            <span>DNS Records</span>
            <span className="text-xs bg-gray-100 text-gray-600 px-2 py-0.5 rounded-full font-bold ml-1">
              {records.length}
            </span>
          </button>

          <button
            onClick={() => setActiveTab('stream')}
            className={`flex items-center space-x-2 py-3 px-4 font-semibold text-sm border-b-2 transition-all whitespace-nowrap cursor-pointer ${
              activeTab === 'stream'
                ? 'border-green-600 text-green-700 bg-white rounded-t-lg shadow-xs'
                : 'border-transparent text-gray-500 hover:text-gray-800'
            }`}
          >
            <Radio className="w-4 h-4" />
            <span>Live Query Stream</span>
            {wsStatus === 'connected' && (
              <span className="w-2 h-2 rounded-full bg-green-500 animate-pulse ml-1" />
            )}
            <span className="text-xs bg-gray-100 text-gray-600 px-2 py-0.5 rounded-full font-bold ml-1">
              {requests.length}
            </span>
          </button>

          <button
            onClick={() => setActiveTab('tester')}
            className={`flex items-center space-x-2 py-3 px-4 font-semibold text-sm border-b-2 transition-all whitespace-nowrap cursor-pointer ${
              activeTab === 'tester'
                ? 'border-green-600 text-green-700 bg-white rounded-t-lg shadow-xs'
                : 'border-transparent text-gray-500 hover:text-gray-800'
            }`}
          >
            <Terminal className="w-4 h-4" />
            <span>DNS Tester ("Web Dig")</span>
          </button>

          <button
            onClick={() => setActiveTab('experiments')}
            className={`flex items-center space-x-2 py-3 px-4 font-semibold text-sm border-b-2 transition-all whitespace-nowrap cursor-pointer ${
              activeTab === 'experiments'
                ? 'border-green-600 text-green-700 bg-white rounded-t-lg shadow-xs'
                : 'border-transparent text-gray-500 hover:text-gray-800'
            }`}
          >
            <Sparkles className="w-4 h-4 text-purple-600" />
            <span>Experiments & Labs</span>
          </button>

          <button
            onClick={() => setActiveTab('parental')}
            className={`flex items-center space-x-2 py-3 px-4 font-semibold text-sm border-b-2 transition-all whitespace-nowrap cursor-pointer ${
              activeTab === 'parental'
                ? 'border-green-600 text-green-700 bg-white rounded-t-lg shadow-xs'
                : 'border-transparent text-gray-500 hover:text-gray-800'
            }`}
          >
            <Shield className="w-4 h-4 text-emerald-600" />
            <span>Parental Control & Blocker</span>
            <span className="text-[10px] bg-green-100 text-green-700 px-1.5 py-0.5 rounded font-bold ml-1 uppercase">
              New
            </span>
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
              dnsPort={session.dnsPort || 5354}
              onSelectExperiment={handleSelectExperiment}
            />
          )}

          {activeTab === 'parental' && (
            <ParentalControl
              subdomain={session.subdomain!}
              baseDomain={session.baseDomain || 'zcdns.id'}
              dnsPort={session.dnsPort || 5354}
            />
          )}
        </div>
      </div>
    </div>
  );
}
