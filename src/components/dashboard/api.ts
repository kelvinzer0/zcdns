import type { DnsRecord, DnsRequestLog, ParentalConfig, TestQueryResult, UserSession } from './types';

const API_BASE = '/api';

export const dashboardApi = {
  async getSession(subdomain?: string): Promise<UserSession> {
    const headers: Record<string, string> = {};
    if (subdomain) {
      headers['X-Subdomain'] = subdomain;
    }
    const res = await fetch(`${API_BASE}/session`, {
      credentials: 'include',
      headers,
    });
    if (!res.ok) {
      return { logged_in: false };
    }
    return res.json();
  },

  async createSession(): Promise<UserSession> {
    const res = await fetch(`${API_BASE}/session`, {
      method: 'POST',
      credentials: 'include',
    });
    if (!res.ok) {
      const err = await res.json().catch(() => ({ error: 'Failed to create session' }));
      throw new Error(err.error || 'Failed to create session');
    }
    const data = await res.json();
    return { ...data, logged_in: true };
  },

  async renewSubdomain(subdomain: string): Promise<UserSession> {
    const res = await fetch(`${API_BASE}/session/renew`, {
      method: 'POST',
      credentials: 'include',
      headers: { 'X-Subdomain': subdomain },
    });
    if (!res.ok) {
      const err = await res.json().catch(() => ({ error: 'Gagal memperpanjang masa aktif subdomain' }));
      throw new Error(err.error || 'Gagal memperpanjang masa aktif subdomain');
    }
    return res.json();
  },

  async deleteSession(): Promise<void> {
    await fetch(`${API_BASE}/session`, {
      method: 'DELETE',
      credentials: 'include',
    });
  },

  async getRecords(subdomain: string): Promise<DnsRecord[]> {
    const res = await fetch(`${API_BASE}/records`, {
      credentials: 'include',
      headers: { 'X-Subdomain': subdomain },
    });
    if (!res.ok) {
      throw new Error('Failed to fetch records');
    }
    return res.json();
  },

  async createRecord(subdomain: string, record: { name: string; type: string; value: string; ttl: number }): Promise<DnsRecord> {
    const res = await fetch(`${API_BASE}/records`, {
      method: 'POST',
      credentials: 'include',
      headers: {
        'Content-Type': 'application/json',
        'X-Subdomain': subdomain,
      },
      body: JSON.stringify(record),
    });
    if (!res.ok) {
      const err = await res.json().catch(() => ({ error: 'Failed to create record' }));
      throw new Error(err.error || 'Failed to create record');
    }
    return res.json();
  },

  async updateRecord(subdomain: string, id: string, record: { name: string; type: string; value: string; ttl: number }): Promise<DnsRecord> {
    const res = await fetch(`${API_BASE}/records/${id}`, {
      method: 'PUT',
      credentials: 'include',
      headers: {
        'Content-Type': 'application/json',
        'X-Subdomain': subdomain,
      },
      body: JSON.stringify(record),
    });
    if (!res.ok) {
      const err = await res.json().catch(() => ({ error: 'Failed to update record' }));
      throw new Error(err.error || 'Failed to update record');
    }
    return res.json();
  },

  async deleteRecord(subdomain: string, id: string): Promise<void> {
    const res = await fetch(`${API_BASE}/records/${id}`, {
      method: 'DELETE',
      credentials: 'include',
      headers: { 'X-Subdomain': subdomain },
    });
    if (!res.ok) {
      throw new Error('Failed to delete record');
    }
  },

  async deleteAllRecords(subdomain: string): Promise<void> {
    const res = await fetch(`${API_BASE}/records`, {
      method: 'DELETE',
      credentials: 'include',
      headers: { 'X-Subdomain': subdomain },
    });
    if (!res.ok) {
      throw new Error('Failed to clear records');
    }
  },

  async getRequests(subdomain: string): Promise<DnsRequestLog[]> {
    const res = await fetch(`${API_BASE}/requests`, {
      credentials: 'include',
      headers: { 'X-Subdomain': subdomain },
    });
    if (!res.ok) {
      throw new Error('Failed to fetch request logs');
    }
    return res.json();
  },

  async deleteRequests(subdomain: string): Promise<void> {
    const res = await fetch(`${API_BASE}/requests`, {
      method: 'DELETE',
      credentials: 'include',
      headers: { 'X-Subdomain': subdomain },
    });
    if (!res.ok) {
      throw new Error('Failed to clear request logs');
    }
  },

  async testQuery(name: string, type: string, nameserver?: string): Promise<TestQueryResult> {
    const res = await fetch(`${API_BASE}/test-query`, {
      method: 'POST',
      headers: { 'Content-Type': 'application/json' },
      body: JSON.stringify({ name, type, nameserver }),
    });
    if (!res.ok) {
      throw new Error('DNS test query failed');
    }
    return res.json();
  },

  async getParentalConfig(subdomain: string): Promise<ParentalConfig> {
    const res = await fetch(`${API_BASE}/parental`, {
      credentials: 'include',
      headers: { 'X-Subdomain': subdomain },
    });
    if (!res.ok) {
      throw new Error('Failed to fetch parental config');
    }
    return res.json();
  },

  async saveParentalConfig(subdomain: string, config: ParentalConfig): Promise<ParentalConfig> {
    const res = await fetch(`${API_BASE}/parental`, {
      method: 'POST',
      credentials: 'include',
      headers: {
        'Content-Type': 'application/json',
        'X-Subdomain': subdomain,
      },
      body: JSON.stringify(config),
    });
    if (!res.ok) {
      throw new Error('Failed to save parental config');
    }
    return res.json();
  },

  connectWebSocket(subdomain: string, onMessage: (log: DnsRequestLog) => void, onStatusChange: (status: 'connected' | 'disconnected' | 'connecting') => void): () => void {
    const protocol = window.location.protocol === 'https:' ? 'wss:' : 'ws:';
    const host = window.location.host;
    const wsUrl = `${protocol}//${host}/requeststream?subdomain=${encodeURIComponent(subdomain)}`;

    let ws: WebSocket | null = null;
    let isClosedExplicitly = false;
    let reconnectTimeout: ReturnType<typeof setTimeout> | null = null;

    const connect = () => {
      if (isClosedExplicitly) return;
      onStatusChange('connecting');

      ws = new WebSocket(wsUrl);

      ws.onopen = () => {
        onStatusChange('connected');
      };

      ws.onmessage = (event) => {
        try {
          const logData = JSON.parse(event.data);
          onMessage(logData);
        } catch (e) {
          // Ignore non-JSON (like ping)
        }
      };

      ws.onclose = () => {
        if (!isClosedExplicitly) {
          onStatusChange('disconnected');
          reconnectTimeout = setTimeout(connect, 2000);
        }
      };

      ws.onerror = () => {
        ws?.close();
      };
    };

    connect();

    return () => {
      isClosedExplicitly = true;
      if (reconnectTimeout) clearTimeout(reconnectTimeout);
      ws?.close();
    };
  },
};
