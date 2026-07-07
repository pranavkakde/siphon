import { useState, useEffect, useRef } from 'react';
import { AreaChart, Area, BarChart, Bar, LineChart, Line, XAxis, YAxis, Tooltip, ResponsiveContainer, Legend, PieChart, Pie, Cell } from 'recharts';
import { Activity, Clock, Image as ImageIcon, Search, BarChart3, Database, ChevronRight, LayoutGrid, Terminal, Cpu, Settings, Brain, AlertTriangle, CheckCircle2, Zap, ShieldAlert, RefreshCw, Eye, EyeOff, Save, X } from 'lucide-react';
import './App.css';

interface TestStep {
  name: string;
  status: string;
  duration_ms: number;
}

interface AIAnalysis {
  category: 'Locator_Changed' | 'API_Failure' | 'Data_Stale' | 'Environment_Issue';
  confidence: number;
  root_cause: string;
  suggested_fix: string;
  analyzed_at: string;
  model: string;
  provider: string;
}

interface TestRun {
  execution_id: string;
  test_case_id: string;
  suite_id: string;
  suite_name: string;
  environment: string;
  project: string;
  release: string;
  sprint: string;
  timestamp: string;
  test_case_name: string;
  status: string;
  duration_ms: number;
  error_message?: string;
  screenshot_url?: string;
  steps?: TestStep[];
  dom_snapshot?: string;
  har_data?: string;
  error_trace?: string;
  ai_status?: 'pending' | 'done' | 'error';
  ai_analysis?: AIAnalysis;
  ai_error?: string;
}

interface ProjectMetric {
  _id: string;
  avg_duration: number;
  total: number;
  passed: number;
}

interface SprintMetric {
  _id: string;
  avg_duration: number;
  total: number;
  passed: number;
}

interface LLMSettings {
  provider: string;
  model: string;
  base_url: string;
  has_key: boolean;
}

const API_URL = import.meta.env.VITE_API_URL || 'http://localhost:8080';
const WS_URL = import.meta.env.VITE_WS_URL || 'ws://localhost:8080';

// ─── AI Category Config ────────────────────────────────────────────────────────
const AI_CATEGORY_CONFIG: Record<string, { label: string; color: string; icon: React.ReactNode; description: string }> = {
  'Locator_Changed': {
    label: 'Locator Changed',
    color: 'var(--ai-orange)',
    icon: <AlertTriangle size={14} />,
    description: 'Selector or DOM structure has changed'
  },
  'API_Failure': {
    label: 'API Failure',
    color: 'var(--color-fail)',
    icon: <ShieldAlert size={14} />,
    description: 'Network or backend API error'
  },
  'Data_Stale': {
    label: 'Data Stale',
    color: 'var(--ai-yellow)',
    icon: <RefreshCw size={14} />,
    description: 'Test data is outdated or expired'
  },
  'Environment_Issue': {
    label: 'Environment Issue',
    color: 'var(--ai-purple)',
    icon: <Zap size={14} />,
    description: 'Infrastructure or config problem'
  }
};

const PROVIDER_OPTIONS = [
  { value: 'openai', label: 'OpenAI', defaultModel: 'gpt-4o-mini', needsKey: true },
  { value: 'anthropic', label: 'Anthropic', defaultModel: 'claude-3-5-haiku-20241022', needsKey: true },
  { value: 'openai_compatible', label: 'OpenAI-Compatible (Ollama, etc.)', defaultModel: 'llama3', needsKey: false },
];

// ─── Settings Modal ─────────────────────────────────────────────────────────
function SettingsModal({ onClose }: { onClose: () => void }) {
  const [settings, setSettings] = useState<LLMSettings>({ provider: 'openai', model: 'gpt-4o-mini', base_url: '', has_key: false });
  const [apiKey, setApiKey] = useState('');
  const [showKey, setShowKey] = useState(false);
  const [saving, setSaving] = useState(false);
  const [saved, setSaved] = useState(false);
  const [loading, setLoading] = useState(true);

  useEffect(() => {
    fetch(`${API_URL}/api/settings`)
      .then(r => r.json())
      .then(data => { setSettings(data); setLoading(false); })
      .catch(() => setLoading(false));
  }, []);

  const selectedProvider = PROVIDER_OPTIONS.find(p => p.value === settings.provider) || PROVIDER_OPTIONS[0];

  const handleProviderChange = (value: string) => {
    const p = PROVIDER_OPTIONS.find(opt => opt.value === value);
    setSettings(prev => ({ ...prev, provider: value, model: p?.defaultModel || '' }));
  };

  const handleSave = async () => {
    setSaving(true);
    try {
      await fetch(`${API_URL}/api/settings`, {
        method: 'POST',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify({
          provider: settings.provider,
          api_key: apiKey || (settings.has_key ? '••••••••' : ''),
          model: settings.model,
          base_url: settings.base_url,
        }),
      });
      setSaved(true);
      setApiKey('');
      setSettings(prev => ({ ...prev, has_key: prev.has_key || apiKey.length > 0 }));
      setTimeout(() => setSaved(false), 2500);
    } catch {
      /* ignore */
    } finally {
      setSaving(false);
    }
  };

  return (
    <div className="modal-overlay" onClick={onClose}>
      <div className="settings-modal" onClick={e => e.stopPropagation()}>
        <div className="settings-modal-header">
          <div className="settings-modal-title">
            <Brain size={20} />
            <span>AI Analyzer Settings</span>
          </div>
          <button className="modal-close-btn" onClick={onClose}><X size={18} /></button>
        </div>

        {loading ? (
          <div className="settings-loading">
            <div className="shimmer-line" style={{ width: '60%' }}></div>
            <div className="shimmer-line" style={{ width: '80%' }}></div>
          </div>
        ) : (
          <div className="settings-form">
            <div className="settings-info-banner">
              <Brain size={14} />
              <span>Configure your LLM provider to enable automatic failure categorization and fix suggestions on all FAIL results.</span>
            </div>

            {/* Provider Selector */}
            <div className="settings-field">
              <label className="settings-label">LLM Provider</label>
              <div className="provider-grid">
                {PROVIDER_OPTIONS.map(opt => (
                  <button
                    key={opt.value}
                    className={`provider-btn ${settings.provider === opt.value ? 'active' : ''}`}
                    onClick={() => handleProviderChange(opt.value)}
                  >
                    {opt.label}
                  </button>
                ))}
              </div>
            </div>

            {/* API Key */}
            {selectedProvider.needsKey && (
              <div className="settings-field">
                <label className="settings-label">
                  API Key
                  {settings.has_key && <span className="key-badge">● Configured</span>}
                </label>
                <div className="key-input-wrap">
                  <input
                    type={showKey ? 'text' : 'password'}
                    className="settings-input"
                    placeholder={settings.has_key ? '••••••••  (leave blank to keep current)' : 'Enter your API key...'}
                    value={apiKey}
                    onChange={e => setApiKey(e.target.value)}
                    id="llm-api-key-input"
                  />
                  <button className="key-toggle-btn" onClick={() => setShowKey(v => !v)}>
                    {showKey ? <EyeOff size={15} /> : <Eye size={15} />}
                  </button>
                </div>
                <p className="settings-hint">Stored securely in MongoDB. Never logged or exposed in responses.</p>
              </div>
            )}

            {/* Base URL (for openai_compatible) */}
            {settings.provider === 'openai_compatible' && (
              <div className="settings-field">
                <label className="settings-label">Base URL</label>
                <input
                  type="text"
                  className="settings-input"
                  placeholder="http://localhost:11434  (Ollama default)"
                  value={settings.base_url}
                  onChange={e => setSettings(prev => ({ ...prev, base_url: e.target.value }))}
                />
              </div>
            )}

            {/* Model */}
            <div className="settings-field">
              <label className="settings-label">Model</label>
              <input
                type="text"
                className="settings-input"
                placeholder={selectedProvider.defaultModel}
                value={settings.model}
                onChange={e => setSettings(prev => ({ ...prev, model: e.target.value }))}
              />
            </div>

            <button
              className={`settings-save-btn ${saved ? 'saved' : ''}`}
              onClick={handleSave}
              disabled={saving}
            >
              {saved ? <><CheckCircle2 size={16} /> Saved!</> : saving ? 'Saving...' : <><Save size={16} /> Save Settings</>}
            </button>
          </div>
        )}
      </div>
    </div>
  );
}

// ─── AI Insights Panel ─────────────────────────────────────────────────────
function AIInsightsPanel({ run }: { run: TestRun }) {
  const [open, setOpen] = useState(false);

  if (run.status !== 'FAIL') return null;

  const ai = run.ai_analysis;
  const status = run.ai_status;
  const catConfig = ai ? AI_CATEGORY_CONFIG[ai.category] : null;

  return (
    <div className="ai-insights-wrapper">
      <button
        className={`ai-badge-btn ${status === 'done' ? 'done' : status === 'error' ? 'error' : 'pending'}`}
        onClick={() => setOpen(v => !v)}
        id={`ai-insights-btn-${run.execution_id}-${run.test_case_id}`}
      >
        <Brain size={13} />
        {status === 'done' && ai
          ? <span>AI: <strong>{catConfig?.label}</strong></span>
          : status === 'error'
          ? <span>AI Analysis Error</span>
          : <span>Analyzing...</span>
        }
        <ChevronRight size={12} className={`ai-chevron ${open ? 'open' : ''}`} />
      </button>

      {open && (
        <div className="ai-panel">
          {status === 'pending' && (
            <div className="ai-shimmer-state">
              <div className="ai-spinner" />
              <div className="ai-shimmer-lines">
                <div className="shimmer-line" style={{ width: '55%' }}></div>
                <div className="shimmer-line" style={{ width: '85%' }}></div>
                <div className="shimmer-line" style={{ width: '70%' }}></div>
              </div>
              <p className="ai-shimmer-label">LLM analysis in progress...</p>
            </div>
          )}

          {status === 'error' && (
            <div className="ai-error-state">
              <ShieldAlert size={18} />
              <p>Analysis failed: {run.ai_error || 'LLM provider not configured.'}</p>
              <p className="ai-error-hint">Configure your API key in <strong>Settings</strong> to enable AI analysis.</p>
            </div>
          )}

          {status === 'done' && ai && catConfig && (
            <div className="ai-result">
              {/* Category + Confidence */}
              <div className="ai-result-header">
                <span className="ai-category-pill" style={{ background: catConfig.color + '22', color: catConfig.color, borderColor: catConfig.color + '44' }}>
                  {catConfig.icon}
                  {catConfig.label}
                </span>
                <div className="ai-confidence">
                  <span className="ai-confidence-label">Confidence</span>
                  <div className="ai-confidence-bar-track">
                    <div
                      className="ai-confidence-bar-fill"
                      style={{
                        width: `${Math.round(ai.confidence * 100)}%`,
                        background: ai.confidence > 0.8 ? 'var(--color-pass)' : ai.confidence > 0.6 ? 'var(--ai-yellow)' : 'var(--color-fail)'
                      }}
                    />
                  </div>
                  <span className="ai-confidence-pct">{Math.round(ai.confidence * 100)}%</span>
                </div>
              </div>

              {/* Root Cause */}
              <div className="ai-section">
                <div className="ai-section-title">
                  <Brain size={13} />
                  Root Cause
                </div>
                <p className="ai-root-cause">{ai.root_cause}</p>
              </div>

              {/* Suggested Fix */}
              <div className="ai-section">
                <div className="ai-section-title">
                  <CheckCircle2 size={13} />
                  Suggested Fix
                </div>
                <pre className="ai-fix-code">{ai.suggested_fix}</pre>
              </div>

              {/* Footer metadata */}
              <div className="ai-meta-footer">
                <span>Model: <strong>{ai.model}</strong></span>
                <span>Provider: <strong>{ai.provider}</strong></span>
                <span>Analyzed: <strong>{new Date(ai.analyzed_at).toLocaleTimeString()}</strong></span>
              </div>
            </div>
          )}
        </div>
      )}
    </div>
  );
}

// ─── Main App ──────────────────────────────────────────────────────────────
export default function App() {
  const [runs, setRuns] = useState<TestRun[]>([]);
  const [projectMetrics, setProjectMetrics] = useState<ProjectMetric[]>([]);
  const [sprintMetrics, setSprintMetrics] = useState<SprintMetric[]>([]);
  const [aiCategoryDist, setAICategoryDist] = useState<{ _id: string; count: number }[]>([]);
  
  const [options, setOptions] = useState({
    projects: [] as string[],
    releases: [] as string[],
    sprints: [] as string[]
  });

  const [filters, setFilters] = useState({
    project: '', // Active selected project from sidebar
    release: '',
    sprint: '',
    search: ''
  });

  const [stats, setStats] = useState({
    total: 0,
    passed: 0,
    failed: 0,
    skipped: 0,
    durationAvg: 0,
    totalExecutions: 0
  });

  const [connected, setConnected] = useState(false);
  const [activeTab, setActiveTab] = useState<'stream' | 'analytics' | 'settings'>('stream');
  const [showSettingsModal, setShowSettingsModal] = useState(false);

  const filtersRef = useRef(filters);
  useEffect(() => {
    filtersRef.current = filters;
  }, [filters]);

  useEffect(() => {
    fetchData();
  }, [filters]);

  useEffect(() => {
    const ws = new WebSocket(`${WS_URL}/ws`);
    ws.onopen = () => setConnected(true);
    ws.onclose = () => setConnected(false);
    ws.onmessage = (event) => {
      try {
        const newRun: TestRun = JSON.parse(event.data);
        const currentFilters = filtersRef.current;
        
        const matchesFilter = 
          (!currentFilters.project || newRun.project === currentFilters.project) &&
          (!currentFilters.release || newRun.release === currentFilters.release) &&
          (!currentFilters.sprint || newRun.sprint === currentFilters.sprint) &&
          (!currentFilters.search || 
            newRun.test_case_name.toLowerCase().includes(currentFilters.search.toLowerCase()) ||
            (newRun.error_message && newRun.error_message.toLowerCase().includes(currentFilters.search.toLowerCase())) ||
            newRun.suite_name.toLowerCase().includes(currentFilters.search.toLowerCase()));

        if (matchesFilter) {
          setRuns(prev => {
            const index = prev.findIndex(r => r.execution_id === newRun.execution_id && r.test_case_id === newRun.test_case_id);
            let nextRuns = [...prev];
            if (index !== -1) {
              nextRuns[index] = newRun;
            } else {
              nextRuns = [newRun, ...prev];
            }
            return nextRuns.slice(0, 100);
          });
          
          fetchStats(currentFilters);
        }
      } catch (err) {
        console.error("WS error: ", err);
      }
    };

    return () => ws.close();
  }, []);

  const fetchData = (currentFilters = filters) => {
    const params = new URLSearchParams({
      project: currentFilters.project,
      release: currentFilters.release,
      sprint: currentFilters.sprint,
      search: currentFilters.search
    }).toString();

    fetch(`${API_URL}/api/runs?${params}`)
      .then(res => res.json())
      .then((data: TestRun[]) => {
        if (Array.isArray(data)) {
          setRuns(data);
        }
      })
      .catch(err => console.error("Error fetching runs:", err));

    fetchStats(currentFilters);
  };

  const fetchStats = (currentFilters = filters) => {
    const params = new URLSearchParams({
      project: currentFilters.project,
      release: currentFilters.release,
      sprint: currentFilters.sprint,
      search: currentFilters.search
    }).toString();

    fetch(`${API_URL}/api/stats?${params}`)
      .then(res => res.json())
      .then((data) => {
        if (data) {
          const distribution = data.status_distribution || [];
          let total = 0;
          let passed = 0;
          let failed = 0;
          let skipped = 0;

          distribution.forEach((item: any) => {
            total += item.count;
            if (item._id === 'PASS') passed = item.count;
            if (item._id === 'FAIL') failed = item.count;
            if (item._id === 'SKIPPED') skipped = item.count;
          });

          setStats({
            total,
            passed,
            failed,
            skipped,
            durationAvg: data.avg_duration || 0,
            totalExecutions: data.total_executions || 0
          });

          if (data.project_metrics) setProjectMetrics(data.project_metrics);
          if (data.sprint_metrics) setSprintMetrics(data.sprint_metrics);
          if (data.ai_category_dist) setAICategoryDist(data.ai_category_dist);

          if (data.filter_options) {
            setOptions({
              projects: data.filter_options.projects || [],
              releases: data.filter_options.releases || [],
              sprints: data.filter_options.sprints || []
            });
          }
        }
      })
      .catch(err => console.error("Error fetching stats:", err));
  };

  const handleFilterChange = (key: string, value: string) => {
    setFilters(prev => ({ ...prev, [key]: value }));
  };

  const clearFilters = () => {
    setFilters({
      project: '',
      release: '',
      sprint: '',
      search: ''
    });
  };

  const pieData = [
    { name: 'Passed', value: stats.passed, color: 'var(--color-pass)' },
    { name: 'Failed', value: stats.failed, color: 'var(--color-fail)' },
    { name: 'Skipped', value: stats.skipped, color: 'var(--color-skipped)' }
  ].filter(d => d.value > 0);

  const aiPieData = aiCategoryDist
    .filter(d => d._id)
    .map(d => ({
      name: AI_CATEGORY_CONFIG[d._id]?.label || d._id,
      value: d.count,
      color: AI_CATEGORY_CONFIG[d._id]?.color || '#888'
    }));

  const getLatencyChartData = () => {
    return [...runs]
      .reverse()
      .slice(-15)
      .map(r => ({
        name: r.test_case_name.substring(0, 10) + '...',
        duration: r.duration_ms,
      }));
  };

  return (
    <div className="layout-wrapper">
      {/* Settings Modal */}
      {showSettingsModal && <SettingsModal onClose={() => setShowSettingsModal(false)} />}

      {/* Collapsable Left Sidebar */}
      <aside className="sidebar">
        <div className="sidebar-brand">
          <img src="/siphon_logo.png" alt="Siphon Logo" className="brand-logo" />
          <div className="brand-text">
            <h2>SIPHON</h2>
            <span>Test Analytics Hub</span>
          </div>
        </div>

        {/* Tab Selection Navigation */}
        <div className="sidebar-nav">
          <button 
            className={`nav-item ${activeTab === 'stream' ? 'active' : ''}`}
            onClick={() => setActiveTab('stream')}
          >
            <Database size={16} />
            <span>Activity Feed</span>
            <ChevronRight size={14} className="chevron" />
          </button>
          <button 
            className={`nav-item ${activeTab === 'analytics' ? 'active' : ''}`}
            onClick={() => setActiveTab('analytics')}
          >
            <BarChart3 size={16} />
            <span>Advanced Analytics</span>
            <ChevronRight size={14} className="chevron" />
          </button>
          <button
            className={`nav-item ${activeTab === 'settings' ? 'active' : ''}`}
            onClick={() => setShowSettingsModal(true)}
            id="open-settings-btn"
          >
            <Settings size={16} />
            <span>AI Settings</span>
            <ChevronRight size={14} className="chevron" />
          </button>
        </div>

        {/* Project-Wise Results Tree Section */}
        <div className="sidebar-section">
          <span className="section-title">Projects</span>
          <div className="project-tree-list">
            <button 
              className={`project-tree-item ${filters.project === '' ? 'selected' : ''}`}
              onClick={() => handleFilterChange('project', '')}
            >
              <LayoutGrid size={14} />
              <span>All Projects</span>
            </button>
            {[...projectMetrics]
              .filter(p => p._id) // Filter out any empty/null project keys
              .sort((a, b) => a._id.localeCompare(b._id))
              .map(p => {
                const passRate = p.total > 0 ? Math.round((p.passed / p.total) * 100) : 0;
                return (
                  <button 
                    key={p._id}
                    className={`project-tree-item ${filters.project === p._id ? 'selected' : ''}`}
                    onClick={() => handleFilterChange('project', p._id)}
                  >
                    <div className="project-item-header">
                      <span className="project-name">{p._id}</span>
                      <span className={`project-rate-badge ${passRate >= 80 ? 'pass' : 'fail'}`}>
                        {passRate}%
                      </span>
                    </div>
                    <div className="project-item-sub">
                      <span>Runs: <strong>{p.total}</strong></span>
                      <span>Speed: <strong>{Math.round(p.avg_duration)}ms</strong></span>
                    </div>
                  </button>
                );
              })}
          </div>
        </div>

        {/* Observation links */}
        <div className="sidebar-section footer-section">
          <span className="section-title">Telemetry Consoles</span>
          <a href="http://localhost:3000" target="_blank" rel="noreferrer" className="console-link">
            <Cpu size={14} /> Grafana Dashboard
          </a>
          <a href="http://localhost:15672" target="_blank" rel="noreferrer" className="console-link">
            <Terminal size={14} /> RabbitMQ Management
          </a>
          <a href="http://localhost:9001" target="_blank" rel="noreferrer" className="console-link">
            <ImageIcon size={14} /> MinIO Web Console
          </a>
        </div>
      </aside>

      {/* Main Content Pane Workspace */}
      <main className="main-content-pane">
        <header className="main-header">
          <div className="header-info">
            <h2>{filters.project ? `Project: ${filters.project}` : 'Global Workspace'}</h2>
            <p>Real-time metrics pipeline execution dashboard</p>
          </div>

          <div style={{ display: 'flex', alignItems: 'center', gap: '0.75rem' }}>
            <button className="header-settings-btn" onClick={() => setShowSettingsModal(true)} id="header-settings-btn">
              <Brain size={15} />
              AI Settings
            </button>
            <div className="status-indicator" style={{ 
              color: connected ? 'var(--color-pass)' : 'var(--color-fail)',
              background: connected ? 'rgba(16, 185, 129, 0.1)' : 'rgba(239, 68, 68, 0.1)',
              borderColor: connected ? 'rgba(16, 185, 129, 0.2)' : 'rgba(239, 68, 68, 0.2)'
            }}>
              <span className="dot" style={{ backgroundColor: connected ? 'var(--color-pass)' : 'var(--color-fail)' }}></span>
              {connected ? 'Pipeline Connected' : 'Pipeline Reconnecting'}
            </div>
          </div>
        </header>

        {/* Global Query Search/Filtering bar */}
        <section className="filter-bar">
          <div className="search-box">
            <Search size={16} className="search-icon" />
            <input 
              type="text" 
              placeholder="Search test names, error logs, suite IDs..." 
              value={filters.search}
              onChange={(e) => handleFilterChange('search', e.target.value)}
            />
          </div>
          <div className="dropdowns-group">
            <div className="select-wrapper">
              <select value={filters.release} onChange={(e) => handleFilterChange('release', e.target.value)}>
                <option value="">All Releases</option>
                {options.releases.map(r => <option key={r} value={r}>{r}</option>)}
              </select>
            </div>
            <div className="select-wrapper">
              <select value={filters.sprint} onChange={(e) => handleFilterChange('sprint', e.target.value)}>
                <option value="">All Sprints</option>
                {options.sprints.map(s => <option key={s} value={s}>{s}</option>)}
              </select>
            </div>
            {(filters.project || filters.release || filters.sprint || filters.search) && (
              <button className="clear-btn" onClick={clearFilters}>Reset Filters</button>
            )}
          </div>
        </section>

        {/* Aggregate Stats Cards */}
        <section className="stats-grid">
          <div className="stat-card">
            <div className="stat-label">Total Test Rows</div>
            <div className="stat-value">{stats.total}</div>
          </div>
          <div className="stat-card" style={{ borderLeft: '4px solid var(--color-pass)' }}>
            <div className="stat-label">Passed</div>
            <div className="stat-value" style={{ color: 'var(--color-pass)' }}>{stats.passed}</div>
          </div>
          <div className="stat-card" style={{ borderLeft: '4px solid var(--color-fail)' }}>
            <div className="stat-label">Failed</div>
            <div className="stat-value" style={{ color: 'var(--color-fail)' }}>{stats.failed}</div>
          </div>
          <div className="stat-card" style={{ borderLeft: '4px solid var(--color-skipped)' }}>
            <div className="stat-label">Skipped</div>
            <div className="stat-value" style={{ color: 'var(--color-skipped)' }}>{stats.skipped}</div>
          </div>
          <div className="stat-card">
            <div className="stat-label">Total Suite Runs</div>
            <div className="stat-value">{stats.totalExecutions}</div>
          </div>
        </section>

        {/* Tab Workspaces */}
        {activeTab === 'stream' ? (
          <div className="main-content">
            <section className="panel">
              <h2 className="panel-title">
                Live Test Execution Activity Feed
                <Activity size={18} style={{ color: 'var(--text-secondary)' }} />
              </h2>

              <div className="test-feed">
                {runs.length === 0 ? (
                  <p style={{ color: 'var(--text-secondary)', textAlign: 'center', padding: '3rem' }}>
                    No execution runs match selected search criteria.
                  </p>
                ) : (
                  runs.map((run) => (
                    <div className="test-item" key={`${run.execution_id}-${run.test_case_id}`}>
                      <div className="test-header">
                        <div className="test-title-area">
                          <h3>{run.test_case_name}</h3>
                          <div className="test-meta">
                            <span>Project: <strong>{run.project || 'N/A'}</strong></span>
                            <span>Release: <strong>{run.release || 'N/A'}</strong></span>
                            <span>Sprint: <strong>{run.sprint || 'N/A'}</strong></span>
                            <span>Env: <strong>{run.environment}</strong></span>
                            <span>Exec ID: <strong>{run.execution_id}</strong></span>
                            <span>Duration: <strong>{run.duration_ms} ms</strong></span>
                          </div>
                        </div>
                        <div>
                          <span className={`badge badge-${run.status.toLowerCase()}`}>
                            {run.status}
                          </span>
                        </div>
                      </div>

                      {run.error_message && (
                        <div className="error-banner">
                          {run.error_message}
                        </div>
                      )}

                      {/* AI Insights Panel (FAIL only) */}
                      <AIInsightsPanel run={run} />

                      {run.screenshot_url && run.status === 'FAIL' && (
                        <div className="screenshot-container">
                          <div style={{ display: 'flex', alignItems: 'center', gap: '0.5rem', padding: '0.5rem', background: 'rgba(255,255,255,0.03)', fontSize: '0.75rem', color: 'var(--text-secondary)' }}>
                            <ImageIcon size={14} />
                            Failure Screenshot from MinIO
                          </div>
                          <img src={run.screenshot_url} alt="MinIO failure trace screenshot" />
                        </div>
                      )}

                      {run.steps && run.steps.length > 0 && (
                        <div className="step-list">
                          {run.steps.map((step: TestStep, idx: number) => (
                            <div className="step-item" key={idx}>
                              <span>
                                <span style={{ 
                                  color: step.status === 'PASS' ? 'var(--color-pass)' : 'var(--color-fail)',
                                  marginRight: '0.5rem' 
                                }}>
                                  {step.status === 'PASS' ? '✓' : '✗'}
                                </span>
                                {step.name}
                              </span>
                              <span style={{ fontSize: '0.75rem' }}>{step.duration_ms} ms</span>
                            </div>
                          ))}
                        </div>
                      )}
                    </div>
                  ))
                )}
              </div>
            </section>

            <section style={{ display: 'flex', flexDirection: 'column', gap: '2rem' }}>
              <div className="panel">
                <h2 className="panel-title">
                  Latency Speed (Last 15 items)
                  <Clock size={18} style={{ color: 'var(--text-secondary)' }} />
                </h2>
                <div style={{ width: '100%', height: '220px', marginTop: '1rem' }}>
                  {runs.length === 0 ? (
                    <p style={{ color: 'var(--text-secondary)', textAlign: 'center', paddingTop: '4rem' }}>
                      No latency data available
                    </p>
                  ) : (
                    <ResponsiveContainer width="100%" height="100%">
                      <AreaChart data={getLatencyChartData()}>
                        <defs>
                          <linearGradient id="colorDuration" x1="0" y1="0" x2="0" y2="1">
                            <stop offset="5%" stopColor="#8b5cf6" stopOpacity={0.4}/>
                            <stop offset="95%" stopColor="#8b5cf6" stopOpacity={0}/>
                          </linearGradient>
                        </defs>
                        <XAxis dataKey="name" tick={{ fill: 'var(--text-secondary)', fontSize: 9 }} />
                        <YAxis tick={{ fill: 'var(--text-secondary)', fontSize: 10 }} />
                        <Tooltip contentStyle={{ background: '#111827', borderColor: 'var(--border-glass)', borderRadius: '8px' }} />
                        <Area type="monotone" dataKey="duration" stroke="#8b5cf6" fillOpacity={1} fill="url(#colorDuration)" />
                      </AreaChart>
                    </ResponsiveContainer>
                  )}
                </div>
              </div>
            </section>
          </div>
        ) : (
          <div className="analytics-workspace">
            <div className="charts-grid-row">
              <div className="panel chart-container">
                <h3 className="panel-title">Pass Rate Distribution</h3>
                <div style={{ width: '100%', height: '240px', display: 'flex', justifyContent: 'center', alignItems: 'center' }}>
                  {stats.total === 0 ? (
                    <p style={{ color: 'var(--text-secondary)' }}>No statistics records matched</p>
                  ) : (
                    <ResponsiveContainer width="100%" height="100%">
                      <PieChart>
                        <Pie
                          data={pieData}
                          cx="50%"
                          cy="50%"
                          innerRadius={60}
                          outerRadius={80}
                          paddingAngle={5}
                          dataKey="value"
                        >
                          {pieData.map((entry, idx) => (
                            <Cell key={`cell-${idx}`} fill={entry.color} />
                          ))}
                        </Pie>
                        <Tooltip contentStyle={{ background: '#111827', borderColor: 'var(--border-glass)', borderRadius: '8px' }} />
                        <Legend verticalAlign="bottom" height={36} />
                      </PieChart>
                    </ResponsiveContainer>
                  )}
                </div>
              </div>

              <div className="panel chart-container">
                <h3 className="panel-title">Average Latency per Project (ms)</h3>
                <div style={{ width: '100%', height: '240px', marginTop: '1rem' }}>
                  {projectMetrics.length === 0 ? (
                    <p style={{ color: 'var(--text-secondary)', textAlign: 'center', paddingTop: '4rem' }}>No project metrics available</p>
                  ) : (
                    <ResponsiveContainer width="100%" height="100%">
                      <BarChart data={
                        filters.project 
                          ? projectMetrics.filter(p => p._id === filters.project)
                          : projectMetrics
                      }>
                        <XAxis dataKey="_id" tick={{ fill: 'var(--text-secondary)', fontSize: 10 }} interval={0} />
                        <YAxis tick={{ fill: 'var(--text-secondary)', fontSize: 10 }} />
                        <Tooltip contentStyle={{ background: '#111827', borderColor: 'var(--border-glass)', borderRadius: '8px' }} />
                        <Bar dataKey="avg_duration" fill="#3b82f6" radius={[4, 4, 0, 0]} name="Average Speed (ms)" />
                      </BarChart>
                    </ResponsiveContainer>
                  )}
                </div>
              </div>
            </div>

            {/* AI Category Distribution Chart */}
            {aiPieData.length > 0 && (
              <div className="charts-grid-row" style={{ marginTop: '2rem' }}>
                <div className="panel chart-container" style={{ flex: '1' }}>
                  <h3 className="panel-title">
                    AI Failure Categorization
                    <Brain size={16} style={{ color: 'var(--ai-purple)' }} />
                  </h3>
                  <div style={{ width: '100%', height: '240px', display: 'flex', justifyContent: 'center', alignItems: 'center' }}>
                    <ResponsiveContainer width="100%" height="100%">
                      <PieChart>
                        <Pie data={aiPieData} cx="50%" cy="50%" innerRadius={50} outerRadius={75} paddingAngle={4} dataKey="value">
                          {aiPieData.map((entry, idx) => <Cell key={idx} fill={entry.color} />)}
                        </Pie>
                        <Tooltip contentStyle={{ background: '#111827', borderColor: 'var(--border-glass)', borderRadius: '8px' }} />
                        <Legend verticalAlign="bottom" height={36} />
                      </PieChart>
                    </ResponsiveContainer>
                  </div>
                </div>

                <div className="panel" style={{ flex: '1' }}>
                  <h3 className="panel-title">AI Category Breakdown</h3>
                  <div style={{ display: 'flex', flexDirection: 'column', gap: '0.75rem', marginTop: '1rem' }}>
                    {aiPieData.map(cat => {
                      const total = aiPieData.reduce((s, d) => s + d.value, 0);
                      const pct = total > 0 ? Math.round((cat.value / total) * 100) : 0;
                      return (
                        <div key={cat.name} style={{ display: 'flex', flexDirection: 'column', gap: '0.3rem' }}>
                          <div style={{ display: 'flex', justifyContent: 'space-between', fontSize: '0.85rem' }}>
                            <span style={{ color: cat.color, fontWeight: 600 }}>{cat.name}</span>
                            <span style={{ color: 'var(--text-secondary)' }}>{cat.value} ({pct}%)</span>
                          </div>
                          <div style={{ height: '4px', background: 'rgba(255,255,255,0.06)', borderRadius: '2px', overflow: 'hidden' }}>
                            <div style={{ height: '100%', width: `${pct}%`, background: cat.color, borderRadius: '2px', transition: 'width 0.6s ease' }} />
                          </div>
                        </div>
                      );
                    })}
                  </div>
                </div>
              </div>
            )}

            <div className="charts-grid-row" style={{ marginTop: '2rem' }}>
              <div className="panel chart-container" style={{ flex: '2' }}>
                <h3 className="panel-title">Sprint Velocity &amp; Pass Ratios</h3>
                <div style={{ width: '100%', height: '260px', marginTop: '1rem' }}>
                  {sprintMetrics.length === 0 ? (
                    <p style={{ color: 'var(--text-secondary)', textAlign: 'center', paddingTop: '5rem' }}>No sprint history</p>
                  ) : (
                    <ResponsiveContainer width="100%" height="100%">
                      <LineChart data={sprintMetrics}>
                        <XAxis dataKey="_id" tick={{ fill: 'var(--text-secondary)', fontSize: 10 }} />
                        <YAxis tick={{ fill: 'var(--text-secondary)', fontSize: 10 }} />
                        <Tooltip contentStyle={{ background: '#111827', borderColor: 'var(--border-glass)', borderRadius: '8px' }} />
                        <Legend />
                        <Line type="monotone" dataKey="total" stroke="#f59e0b" name="Total Test Runs" strokeWidth={2} />
                        <Line type="monotone" dataKey="passed" stroke="#10b981" name="Passed Runs" strokeWidth={2} />
                      </LineChart>
                    </ResponsiveContainer>
                  )}
                </div>
              </div>

              <div className="panel" style={{ flex: '1.2' }}>
                <h3 className="panel-title">Performance Summary</h3>
                <div style={{ display: 'flex', flexDirection: 'column', gap: '1rem', marginTop: '1rem' }}>
                  {(filters.project 
                    ? projectMetrics.filter(p => p._id === filters.project)
                    : projectMetrics
                  ).map(p => {
                    const passRate = p.total > 0 ? Math.round((p.passed / p.total) * 100) : 0;
                    return (
                      <div key={p._id} style={{ display: 'flex', flexDirection: 'column', gap: '0.25rem', borderBottom: '1px solid var(--border-glass)', paddingBottom: '0.75rem' }}>
                        <div style={{ display: 'flex', justifyContent: 'space-between', fontWeight: 'bold' }}>
                          <span>{p._id}</span>
                          <span style={{ color: passRate >= 80 ? 'var(--color-pass)' : 'var(--color-fail)' }}>
                            {passRate}% pass
                          </span>
                        </div>
                        <div style={{ display: 'flex', justifyContent: 'space-between', fontSize: '0.8rem', color: 'var(--text-secondary)' }}>
                          <span>Avg Latency: <strong>{Math.round(p.avg_duration)} ms</strong></span>
                          <span>Total Runs: <strong>{p.total}</strong></span>
                        </div>
                      </div>
                    );
                  })}
                </div>
              </div>
            </div>
          </div>
        )}
      </main>
    </div>
  );
}
