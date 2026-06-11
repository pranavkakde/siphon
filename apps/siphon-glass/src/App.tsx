import { useState, useEffect, useRef } from 'react';
import { AreaChart, Area, BarChart, Bar, LineChart, Line, XAxis, YAxis, Tooltip, ResponsiveContainer, Legend, PieChart, Pie, Cell } from 'recharts';
import { Activity, Clock, Image as ImageIcon, Search, BarChart3, Database, ChevronRight, LayoutGrid, Terminal, Cpu } from 'lucide-react';
import './App.css';

interface TestStep {
  name: string;
  status: string;
  duration_ms: number;
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

export default function App() {
  const [runs, setRuns] = useState<TestRun[]>([]);
  const [projectMetrics, setProjectMetrics] = useState<ProjectMetric[]>([]);
  const [sprintMetrics, setSprintMetrics] = useState<SprintMetric[]>([]);
  
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
  const [activeTab, setActiveTab] = useState<'stream' | 'analytics'>('stream');

  const filtersRef = useRef(filters);
  useEffect(() => {
    filtersRef.current = filters;
  }, [filters]);

  useEffect(() => {
    fetchData();
  }, [filters]);

  useEffect(() => {
    const ws = new WebSocket('ws://localhost:8080/ws');
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

    fetch(`http://localhost:8080/api/runs?${params}`)
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

    fetch(`http://localhost:8080/api/stats?${params}`)
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

          <div className="status-indicator" style={{ 
            color: connected ? 'var(--color-pass)' : 'var(--color-fail)',
            background: connected ? 'rgba(16, 185, 129, 0.1)' : 'rgba(239, 68, 68, 0.1)',
            borderColor: connected ? 'rgba(16, 185, 129, 0.2)' : 'rgba(239, 68, 68, 0.2)'
          }}>
            <span className="dot" style={{ backgroundColor: connected ? 'var(--color-pass)' : 'var(--color-fail)' }}></span>
            {connected ? 'Pipeline Connected' : 'Pipeline Reconnecting'}
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

            <div className="charts-grid-row" style={{ marginTop: '2rem' }}>
              <div className="panel chart-container" style={{ flex: '2' }}>
                <h3 className="panel-title">Sprint Velocity & Pass Ratios</h3>
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
