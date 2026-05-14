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
  const totalFindings = report.categories.reduce((acc, cat) => acc + cat.findings.length, 0);

  // Get active category for findings view
  const activeFindings = selectedCategory 
    ? report.categories.find(c => c.category === selectedCategory)?.findings || []
    : report.correlations || []; // Default to showing correlations if no category selected

  return (
    <div className="dashboard-container">
      <header className="animate-fade-in">
        <div className="logo-section">
          <Database className="text-blue-500" size={32} color="#3b82f6" />
          <h1 className="logo-text">pgaioptimizer</h1>
        </div>
        <div className="flex gap-4">
          <div style={{ display: 'flex', gap: '1rem', color: 'var(--text-secondary)', fontSize: '0.9rem', alignItems: 'center' }}>
            <span>{report.server_info.host}:{report.server_info.port}/{report.server_info.database}</span>
            <span style={{ padding: '2px 8px', borderRadius: '12px', background: 'rgba(255,255,255,0.1)' }}>{report.server_info.version}</span>
          </div>
          <button className="glass-card" style={{ padding: '0.5rem 1rem', display: 'flex', alignItems: 'center', gap: '0.5rem', background: 'rgba(255,255,255,0.05)', marginLeft: '1rem' }}>
            <Settings size={18} />
            <span>Settings</span>
          </button>
        </div>
      </header>

      <main>
        <section className="score-overview">
          <div className="glass-card overall-score-card animate-fade-in">
            <h2 style={{ fontSize: '0.9rem', color: 'var(--text-secondary)', marginBottom: '1.5rem', textTransform: 'uppercase', letterSpacing: '0.1em' }}>Overall Health</h2>
            <div style={{ position: 'relative', width: '200px', height: '200px', display: 'flex', alignItems: 'center', justifyContent: 'center' }}>
              <svg width="200" height="200" viewBox="0 0 200 200">
                <circle cx="100" cy="100" r="80" fill="none" stroke="rgba(255,255,255,0.05)" strokeWidth="12" />
                <circle 
                  cx="100" cy="100" r="80" fill="none" stroke={report.overall_score > 80 ? 'var(--severity-ok)' : report.overall_score > 60 ? 'var(--severity-warning)' : 'var(--severity-critical)'} strokeWidth="12" 
                  strokeDasharray="502.4" strokeDashoffset={502.4 * (1 - report.overall_score/100)} 
                  strokeLinecap="round"
                  style={{ transform: 'rotate(-90deg)', transformOrigin: 'center', transition: 'stroke-dashoffset 1s ease-in-out' }}
                />
                <text x="100" y="105" textAnchor="middle" fontSize="48" fontWeight="bold" fill="white">{Math.round(report.overall_score)}</text>
                <text x="100" y="135" textAnchor="middle" fontSize="16" fill="var(--text-secondary)">Grade {report.grade}</text>
              </svg>
            </div>
            <p style={{ marginTop: '1.5rem', fontSize: '0.9rem', color: 'var(--text-secondary)' }}>
              {totalFindings} issues detected
            </p>
          </div>

          <div className="glass-card animate-fade-in" style={{ padding: '2rem', display: 'flex', flexDirection: 'column', gap: '1.5rem', animationDelay: '0.1s', gridColumn: 'span 2' }}>
            <div style={{ display: 'flex', justifyContent: 'space-between', alignItems: 'center' }}>
              <h2 style={{ fontSize: '1.2rem', fontWeight: '600', display: 'flex', alignItems: 'center', gap: '0.5rem' }}>
                {selectedCategory ? `${selectedCategory.charAt(0).toUpperCase() + selectedCategory.slice(1)} Findings` : 'Cross-Category Root Causes'}
                {selectedCategory && (
                  <button 
                    onClick={() => setSelectedCategory(null)}
                    style={{ fontSize: '0.8rem', padding: '0.2rem 0.5rem', background: 'rgba(255,255,255,0.1)', border: 'none', borderRadius: '4px', color: 'white', cursor: 'pointer', marginLeft: '1rem' }}
                  >
                    Clear Filter
                  </button>
                )}
              </h2>
            </div>
            
            <div style={{ maxHeight: '250px', overflowY: 'auto', paddingRight: '1rem' }}>
              <FindingsList findings={activeFindings} />
            </div>
          </div>
        </section>

        <div className="category-grid">
          {report.categories.map((cat, i) => (
            <div 
              key={cat.category} 
              className="glass-card category-card animate-fade-in" 
              style={{ 
                animationDelay: `${0.2 + i * 0.05}s`, 
                cursor: 'pointer',
                border: selectedCategory === cat.category ? '1px solid var(--accent-blue)' : undefined,
                background: selectedCategory === cat.category ? 'rgba(59, 130, 246, 0.05)' : undefined
              }}
              onClick={() => setSelectedCategory(cat.category)}
            >
              <div className="category-header">
                <div style={{ display: 'flex', alignItems: 'center', gap: '0.75rem' }}>
                  {cat.category === 'configuration' && <Settings size={20} color="var(--accent-blue)" />}
                  {cat.category === 'indexes' && <Zap size={20} color={cat.score > 80 ? 'var(--severity-ok)' : 'var(--severity-critical)'} />}
                  {cat.category === 'queries' && <Search size={20} color={cat.score > 80 ? 'var(--severity-ok)' : 'var(--severity-warning)'} />}
                  {cat.category === 'vacuum' && <Shield size={20} color={cat.score > 80 ? 'var(--severity-ok)' : 'var(--severity-warning)'} />}
                  {cat.category === 'connections' && <Activity size={20} color={cat.score > 80 ? 'var(--severity-ok)' : 'var(--severity-critical)'} />}
                  {cat.category === 'cache' && <Database size={20} color={cat.score > 80 ? 'var(--severity-ok)' : 'var(--severity-critical)'} />}
                  {cat.category === 'locks' && <Shield size={20} color={cat.score > 80 ? 'var(--severity-ok)' : 'var(--severity-critical)'} />}
                  {cat.category === 'sequences' && <Activity size={20} color={cat.score > 80 ? 'var(--severity-ok)' : 'var(--severity-warning)'} />}
                  {cat.category === 'storage' && <Database size={20} color={cat.score > 80 ? 'var(--severity-ok)' : 'var(--severity-warning)'} />}
                  {cat.category === 'replication' && <Activity size={20} color={cat.score > 80 ? 'var(--severity-ok)' : 'var(--severity-warning)'} />}
                  {cat.category === 'schema' && <LayoutDashboard size={20} color={cat.score > 80 ? 'var(--severity-ok)' : 'var(--severity-warning)'} />}
                  {cat.category === 'tables' && <LayoutDashboard size={20} color={cat.score > 80 ? 'var(--severity-ok)' : 'var(--severity-critical)'} />}
                  
                  <span className="category-title" style={{ textTransform: 'capitalize' }}>{cat.category}</span>
                </div>
                <span className={`badge`} style={{ background: cat.score > 80 ? 'var(--severity-ok)' : cat.score > 60 ? 'var(--severity-warning)' : 'var(--severity-critical)' }}>
                  {Math.round(cat.score)}%
                </span>
              </div>
              <div style={{ display: 'flex', justifyContent: 'space-between', alignItems: 'flex-end', marginTop: '1.5rem' }}>
                <span className="finding-count">{cat.findings ? cat.findings.length : 0} findings</span>
                <ChevronRight size={18} color="var(--text-secondary)" />
              </div>
              <div style={{ width: '100%', height: '4px', background: 'rgba(255,255,255,0.05)', marginTop: '0.75rem', borderRadius: '2px', overflow: 'hidden' }}>
                <div style={{ width: `${cat.score}%`, height: '100%', background: cat.score > 80 ? 'var(--severity-ok)' : cat.score > 60 ? 'var(--severity-warning)' : 'var(--severity-critical)' }} />
              </div>
            </div>
          ))}
        </div>
      </main>
    </div>
  );
};

export default App;
