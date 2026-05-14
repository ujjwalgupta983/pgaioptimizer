import React, { useEffect, useState } from 'react';
import { Database, Activity, Shield, Zap, Search, Settings, ChevronRight, LayoutDashboard } from 'lucide-react';
import './styles/index.css';
import { getLatestReport } from './api/client';
import type { HealthReport } from './api/client';
import { FindingsList } from './components/FindingsList';

const App: React.FC = () => {
  const [report, setReport] = useState<HealthReport | null>(null);
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState<string | null>(null);
  const [selectedCategory, setSelectedCategory] = useState<string | null>(null);

  useEffect(() => {
    const fetchReport = async () => {
      try {
        const data = await getLatestReport();
        setReport(data);
      } catch (err: any) {
        setError(err.response?.data?.error || err.message);
      } finally {
        setLoading(false);
      }
    };
    fetchReport();
  }, []);

  if (loading) {
    return <div style={{ display: 'flex', height: '100vh', alignItems: 'center', justifyContent: 'center' }}>Loading pgaioptimizer analysis...</div>;
  }

  if (error || !report) {
    return (
      <div style={{ display: 'flex', flexDirection: 'column', height: '100vh', alignItems: 'center', justifyContent: 'center', gap: '1rem' }}>
        <h2>Backend API Not Found</h2>
        <p style={{ color: 'var(--text-secondary)' }}>Error: {error}</p>
        <p style={{ color: 'var(--text-secondary)' }}>Make sure to run <code>pgaioptimizer agent</code> or <code>pgaioptimizer server</code> and perform a scan.</p>
      </div>
    );
  }

  // Calculate total findings
  const totalFindings = report.categories.reduce((acc, cat) => acc + (cat.findings ? cat.findings.length : 0), 0);

  // Get active category for findings view
  const activeFindings = selectedCategory 
    ? report.categories.find(c => c.category === selectedCategory)?.findings || []
    : report.correlations || []; // Default to showing correlations if no category selected

  return (
    <div className="dashboard-container">
      <header className="animate-fade-in">
        <div className="logo-section">
          <Database className="text-blue-500" size={32} color="var(--accent-blue)" />
          <h1 className="logo-text">pgaioptimizer</h1>
        </div>
        <div className="flex items-center gap-4">
          <div className="flex items-center gap-3 text-secondary text-sm">
            <span className="font-medium">{report.server_info.host}:{report.server_info.port}/{report.server_info.database}</span>
            <span className="badge badge-info">{report.server_info.version.split(' ')[1] || 'PostgreSQL'}</span>
          </div>
          <button className="btn">
            <Settings size={18} />
            <span>Settings</span>
          </button>
        </div>
      </header>

      <main>
        <section className="score-overview">
          <div className="glass-card overall-score-card animate-fade-in">
            <h2 className="text-xs text-secondary mb-6">Overall Health</h2>
            <div className="circular-chart-container">
              <svg className="circular-chart-svg" width="200" height="200" viewBox="0 0 200 200">
                <circle cx="100" cy="100" r="80" className="circular-chart-bg" />
                <circle 
                  cx="100" cy="100" r="80" 
                  className={`circular-chart-path ${report.overall_score < 60 ? 'glow-critical' : ''}`}
                  stroke={report.overall_score > 80 ? 'var(--severity-ok)' : report.overall_score > 60 ? 'var(--severity-warning)' : 'var(--severity-critical)'} 
                  strokeDasharray="502.4" strokeDashoffset={502.4 * (1 - report.overall_score/100)} 
                />
              </svg>
              <div className="circular-chart-text">
                <span style={{ fontSize: '3rem', fontWeight: '800', lineHeight: 1 }}>{Math.round(report.overall_score)}</span>
                <span className="text-sm text-secondary font-medium mt-2">Grade {report.grade}</span>
              </div>
            </div>
            <p className="text-sm text-secondary mt-6">
              <span className="font-bold text-primary">{totalFindings}</span> issues detected
            </p>
          </div>

          <div className="glass-card animate-fade-in" style={{ padding: '2.5rem 2rem', gridColumn: 'span 1', animationDelay: '0.1s' }}>
            <div className="flex justify-between items-center mb-6">
              <h2 className="flex items-center gap-2 font-bold" style={{ fontSize: '1.25rem' }}>
                {selectedCategory ? `${selectedCategory.charAt(0).toUpperCase() + selectedCategory.slice(1)} Findings` : 'Cross-Category Root Causes'}
                {selectedCategory && (
                  <button className="badge badge-info ml-4 cursor-pointer" onClick={() => setSelectedCategory(null)}>
                    Clear Filter
                  </button>
                )}
              </h2>
            </div>
            
            <div className="custom-scrollbar" style={{ maxHeight: '300px', overflowY: 'auto', paddingRight: '1rem' }}>
              <FindingsList findings={activeFindings} />
            </div>
          </div>
        </section>

        <div className="category-grid">
          {report.categories.map((cat, i) => (
            <div 
              key={cat.category} 
              className={`glass-card category-card interactive animate-fade-in ${selectedCategory === cat.category ? 'active' : ''}`}
              style={{ animationDelay: `${0.2 + i * 0.05}s` }}
              onClick={() => setSelectedCategory(cat.category)}
            >
              <div className="category-header">
                <div className="flex items-center gap-3">
                  {cat.category === 'configuration' && <Settings size={22} color="var(--accent-blue)" />}
                  {cat.category === 'indexes' && <Zap size={22} color={cat.score > 80 ? 'var(--severity-ok)' : 'var(--severity-critical)'} />}
                  {cat.category === 'queries' && <Search size={22} color={cat.score > 80 ? 'var(--severity-ok)' : 'var(--severity-warning)'} />}
                  {cat.category === 'vacuum' && <Shield size={22} color={cat.score > 80 ? 'var(--severity-ok)' : 'var(--severity-warning)'} />}
                  {cat.category === 'connections' && <Activity size={22} color={cat.score > 80 ? 'var(--severity-ok)' : 'var(--severity-critical)'} />}
                  {cat.category === 'cache' && <Database size={22} color={cat.score > 80 ? 'var(--severity-ok)' : 'var(--severity-critical)'} />}
                  {cat.category === 'locks' && <Shield size={22} color={cat.score > 80 ? 'var(--severity-ok)' : 'var(--severity-critical)'} />}
                  {cat.category === 'sequences' && <Activity size={22} color={cat.score > 80 ? 'var(--severity-ok)' : 'var(--severity-warning)'} />}
                  {cat.category === 'storage' && <Database size={22} color={cat.score > 80 ? 'var(--severity-ok)' : 'var(--severity-warning)'} />}
                  {cat.category === 'replication' && <Activity size={22} color={cat.score > 80 ? 'var(--severity-ok)' : 'var(--severity-warning)'} />}
                  {cat.category === 'schema' && <LayoutDashboard size={22} color={cat.score > 80 ? 'var(--severity-ok)' : 'var(--severity-warning)'} />}
                  {cat.category === 'tables' && <LayoutDashboard size={22} color={cat.score > 80 ? 'var(--severity-ok)' : 'var(--severity-critical)'} />}
                  
                  <span className="category-title" style={{ textTransform: 'capitalize' }}>{cat.category}</span>
                </div>
                <span className={`badge badge-${cat.score > 80 ? 'ok' : cat.score > 60 ? 'warning' : 'critical'}`}>
                  {Math.round(cat.score)}%
                </span>
              </div>
              <div className="flex justify-between items-center mt-6">
                <span className="finding-count font-medium text-muted">{cat.findings ? cat.findings.length : 0} findings</span>
                <ChevronRight size={18} className="text-secondary" />
              </div>
              <div className="category-progress-bg">
                <div 
                  className="category-progress-fill" 
                  style={{ 
                    width: `${cat.score}%`, 
                    background: cat.score > 80 ? 'var(--severity-ok)' : cat.score > 60 ? 'var(--severity-warning)' : 'var(--severity-critical)' 
                  }} 
                />
              </div>
            </div>
          ))}
        </div>
      </main>
    </div>
  );
};

export default App;
