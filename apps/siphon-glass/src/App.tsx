import { useState, useEffect } from 'react';
import { AreaChart, Area, XAxis, YAxis, Tooltip, ResponsiveContainer } from 'recharts';
import { Activity, Clock, Image as ImageIcon } from 'lucide-react';
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
  timestamp: string;
  test_case_name: string;
  status: string;
  duration_ms: number;
  error_message?: string;
  screenshot_url?: string;
  steps?: TestStep[];
}

export default function App() {
  const [runs, setRuns] = useState<TestRun[]>([]);
  const [stats, setStats] = useState({
    total: 0,
    passed: 0,
    failed: 0,
    skipped: 0,
    durationAvg: 0
  });
  const [connected, setConnected] = useState(false);

  // Fetch initial REST data
  useEffect(() => {
    fetch('http://localhost:8080/api/runs')
      .then(res => res.json())
      .then((data: TestRun[]) => {
        if (Array.isArray(data)) {
          setRuns(data);
          calculateStats(data);
        }
      })
      .catch(err => logError("Error loading recent runs", err));

    // Connect WebSocket
    const ws = new WebSocket('ws://localhost:8080/ws');
    ws.onopen = () => setConnected(true);
    ws.onclose = () => setConnected(false);
    ws.onmessage = (event) => {
      try {
        const newRun: TestRun = JSON.parse(event.data);
        setRuns(prev => {
          // Check for duplication and replace or append
          const index = prev.findIndex(r => r.execution_id === newRun.execution_id && r.test_case_id === newRun.test_case_id);
          let nextRuns = [...prev];
          if (index !== -1) {
            nextRuns[index] = newRun;
          } else {
            nextRuns = [newRun, ...prev];
          }
          calculateStats(nextRuns);
          return nextRuns;
        });
      } catch (err) {
        logError("WS parsing error", err);
      }
    };

    return () => ws.close();
  }, []);

  const logError = (msg: string, err: any) => {
    console.error(msg, err);
  };

  const calculateStats = (data: TestRun[]) => {
    const total = data.length;
    const passed = data.filter(r => r.status === 'PASS').length;
    const failed = data.filter(r => r.status === 'FAIL').length;
    const skipped = data.filter(r => r.status === 'SKIPPED').length;
    const sumDuration = data.reduce((acc, curr) => acc + (curr.duration_ms || 0), 0);
    const durationAvg = total > 0 ? Math.round(sumDuration / total) : 0;

    setStats({ total, passed, failed, skipped, durationAvg });
  };

  // Format data for chart
  const getChartData = () => {
    return [...runs]
      .reverse()
      .slice(-15)
      .map(r => ({
        name: r.test_case_name.substring(0, 15) + '...',
        duration: r.duration_ms,
      }));
  };

  return (
    <div className="dashboard-container">
      <header>
        <div className="logo-section">
          <h1>SIPHON</h1>
          <p>Real-time Test Metrics Analytics Pipeline</p>
        </div>
        <div className="status-section">
          <div className="status-indicator" style={{ 
            color: connected ? 'var(--color-pass)' : 'var(--color-fail)',
            background: connected ? 'rgba(16, 185, 129, 0.1)' : 'rgba(239, 68, 68, 0.1)',
            borderColor: connected ? 'rgba(16, 185, 129, 0.2)' : 'rgba(239, 68, 68, 0.2)'
          }}>
            <span style={{
              width: '8px',
              height: '8px',
              borderRadius: '50%',
              backgroundColor: connected ? 'var(--color-pass)' : 'var(--color-fail)',
              display: 'inline-block'
            }}></span>
            {connected ? 'Pipeline Connected' : 'Pipeline Reconnecting'}
          </div>
        </div>
      </header>

      <section className="stats-grid">
        <div className="stat-card">
          <div className="stat-label">Total Test Cases</div>
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
          <div className="stat-label">Avg Duration</div>
          <div className="stat-value">{stats.durationAvg} ms</div>
        </div>
      </section>

      <div className="main-content">
        <section className="panel">
          <h2 className="panel-title">
            Live Execution Stream Feed
            <Activity size={18} style={{ color: 'var(--text-secondary)' }} />
          </h2>

          <div className="test-feed">
            {runs.length === 0 ? (
              <p style={{ color: 'var(--text-secondary)', textAlign: 'center', padding: '2rem' }}>
                Waiting for incoming gRPC test suite stream executions...
              </p>
            ) : (
              runs.map((run) => (
                <div className="test-item" key={`${run.execution_id}-${run.test_case_id}`}>
                  <div className="test-header">
                    <div className="test-title-area">
                      <h3>{run.test_case_name}</h3>
                      <div className="test-meta">
                        <span>Suite: <strong>{run.suite_name}</strong></span>
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
              Duration Latency Trend
              <Clock size={18} style={{ color: 'var(--text-secondary)' }} />
            </h2>
            <div style={{ width: '100%', height: '220px', marginTop: '1rem' }}>
              {runs.length === 0 ? (
                <p style={{ color: 'var(--text-secondary)', textAlign: 'center', paddingTop: '4rem' }}>
                  No execution latency details yet
                </p>
              ) : (
                <ResponsiveContainer width="100%" height="100%">
                  <AreaChart data={getChartData()}>
                    <defs>
                      <linearGradient id="colorDuration" x1="0" y1="0" x2="0" y2="1">
                        <stop offset="5%" stopColor="#8b5cf6" stopOpacity={0.4}/>
                        <stop offset="95%" stopColor="#8b5cf6" stopOpacity={0}/>
                      </linearGradient>
                    </defs>
                    <XAxis dataKey="name" tick={{ fill: 'var(--text-secondary)', fontSize: 10 }} />
                    <YAxis tick={{ fill: 'var(--text-secondary)', fontSize: 10 }} />
                    <Tooltip contentStyle={{ background: '#111827', borderColor: 'var(--border-glass)', borderRadius: '8px' }} />
                    <Area type="monotone" dataKey="duration" stroke="#8b5cf6" fillOpacity={1} fill="url(#colorDuration)" />
                  </AreaChart>
                </ResponsiveContainer>
              )}
            </div>
          </div>

          <div className="panel" style={{ display: 'flex', flexDirection: 'column', gap: '1rem' }}>
            <h2 className="panel-title" style={{ marginBottom: '0.5rem' }}>Observability Targets</h2>
            <div style={{ fontSize: '0.875rem', display: 'flex', flexDirection: 'column', gap: '0.75rem' }}>
              <div style={{ display: 'flex', justifyContent: 'space-between', borderBottom: '1px solid var(--border-glass)', paddingBottom: '0.5rem' }}>
                <span style={{ color: 'var(--text-secondary)' }}>Grafana Dashboard</span>
                <a href="http://localhost:3000" target="_blank" rel="noreferrer" style={{ color: '#60a5fa', textDecoration: 'none' }}>Go to Grafana</a>
              </div>
              <div style={{ display: 'flex', justifyContent: 'space-between', borderBottom: '1px solid var(--border-glass)', paddingBottom: '0.5rem' }}>
                <span style={{ color: 'var(--text-secondary)' }}>RabbitMQ Management</span>
                <a href="http://localhost:15672" target="_blank" rel="noreferrer" style={{ color: '#60a5fa', textDecoration: 'none' }}>Go to Console</a>
              </div>
              <div style={{ display: 'flex', justifyContent: 'space-between' }}>
                <span style={{ color: 'var(--text-secondary)' }}>MinIO Web UI</span>
                <a href="http://localhost:9001" target="_blank" rel="noreferrer" style={{ color: '#60a5fa', textDecoration: 'none' }}>Go to Bucket Console</a>
              </div>
            </div>
          </div>
        </section>
      </div>
    </div>
  );
}
