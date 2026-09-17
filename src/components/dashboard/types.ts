export type RecordType = 'A' | 'AAAA' | 'CNAME' | 'TXT' | 'MX' | 'NS' | 'PTR' | 'CAA' | 'SRV';

export interface DnsRecord {
  id: string;
  subdomain: string;
  name: string;
  type: RecordType;
  value: string;
  ttl: number;
  created_at: string;
  updated_at: string;
}

export interface DnsRequestLog {
  id: string;
  subdomain: string;
  qname: string;
  qtype: string;
  client_ip: string;
  rcode: string;
  answers: string[];
  created_at: string;
}

export interface UserSession {
  id?: string;
  subdomain?: string;
  domain?: string;
  baseDomain?: string;
  dnsPort?: number;
  logged_in: boolean;
  created_at?: string;
}

export interface TestQueryResult {
  status: 'SUCCESS' | 'ERROR';
  rcode: string;
  question: string;
  answers: string[];
  authority: string[];
  response_time_ms: number;
  server: string;
  raw: string;
}

export interface ParentalConfig {
  subdomain: string;
  enabled: boolean;
  block_adult: boolean;
  block_gambling: boolean;
  block_malware: boolean;
  block_ads: boolean;
  block_social: boolean;
  block_gaming: boolean;
  enforce_safesearch: boolean;
  block_mode: '0.0.0.0' | 'NXDOMAIN';
  custom_blocked: string[];
  custom_allowed: string[];
  updated_at?: string;
}

export interface Experiment {
  id: string;
  title: string;
  difficulty: 'Beginner' | 'Intermediate' | 'Advanced';
  description: string;
  steps: string[];
  suggestedRecord?: {
    name: string;
    type: RecordType;
    value: string;
    ttl: number;
  };
  suggestedQuery?: {
    name: string;
    type: RecordType;
  };
  explanation: string;
}
