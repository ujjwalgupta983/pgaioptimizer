import React from 'react';
import { Database, Activity, Shield, Zap, Search, Settings, AlertCircle, CheckCircle2, ChevronRight } from 'lucide-react';
import './styles/index.css';

// Mock data for demonstration
const categories = [
  { id: 'config', name: 'Configuration', score: 85, findings: 3, severity: 'warning' },
  { id: 'indexes', name: 'Indexes', score: 62, findings: 12, severity: 'critical' },
  { id: 'queries', name: 'Queries', score: 78, findings: 5, severity: 'warning' },
  { id: 'vacuum', name: 'Vacuum & Analyze', score: 92, findings: 1, severity: 'info' },
  { id: 'conn', name: 'Connections', score: 95, findings: 0, severity: 'ok' },
  { id: 'cache', name: 'Cache & Buffers', score: 45, findings: 8, severity: 'critical' },
];

const App: React.FC = () => {
  return (
    <div className="dashboard-container">
      <header className="animate-fade-in">
        <div className="logo-section">
          <Database className="text-blue-500" size={32} color="#3b82f6" />
          <h1 className="logo-text">pgaioptimizer</h1>
        </div>
        <div className="flex gap-4">
          <button className="glass-card" style={{ padding: '0.5rem 1rem', display: 'flex', alignItems: 'center', gap: '0.5rem', background: 'rgba(255,255,255,0.05)' }}>
            <Settings size={18} />
            <span>Settings</span>
          </button>
        </div>
      </header>

      <main>
        <section className="score-overview">
          <div className="glass-card overall-score-card animate-fade-in">
            <h2 style={{ fontSize: '0.9rem', color: 'var(--text-secondary)', marginBottom: '1.5rem', textTransform: 'uppercase', letterSpacing: '0.1em' }}>Overall Health</h2>
            <div style={{ position: 'relative', width: '200px', height: '200px', display: 'flex', alignItems: 'center', justifyCenter: 'center' }}>
              <svg width="200" height="200" viewBox="0 0 200 200">
                <circle cx="100" cy="100" r="80" fill="none" stroke="rgba(255,255,255,0.05)" strokeWidth="12" />
                <circle 
                  cx="100" cy="100" r="80" fill="none" stroke="var(--accent-blue)" strokeWidth="12" 
                  strokeDasharray="502.4" strokeDashoffset={502.4 * (1 - 74/100)} 
                  strokeLinecap="round"
                  style={{ transform: 'rotate(-90deg)', transformOrigin: 'center', transition: 'stroke-dashoffset 1s ease-in-out' }}
                />
                <text x="100" y="105" textAnchor="middle" fontSize="48" fontWeight="bold" fill="white">74</text>
                <text x="100" y="135" textAnchor="middle" fontSize="16" fill="var(--text-secondary)">Grade B</text>
              </svg>
            </div>
            <p style={{ marginTop: '1.5rem', fontSize: '0.9rem', color: 'var(--text-secondary)' }}>
              15 critical issues detected
            </p>
          </div>

          <div className="glass-card animate-fade-in" style={{ padding: '2rem', display: 'flex', flexDirection: 'column', gap: '1.5rem', animationDelay: '0.1s' }}>
            <h2 style={{ fontSize: '1.2rem', fontWeight: '600' }}>Quick Recommendations</h2>
            <div style={{ display: 'flex', flexDirection: 'column', gap: '1rem' }}>
              <div style={{ display: 'flex', gap: '1rem', alignItems: 'flex-start', padding: '1rem', borderRadius: '12px', background: 'rgba(239, 68, 68, 0.1)', borderLeft: '4px solid var(--severity-critical)' }}>
                <AlertCircle color="var(--severity-critical)" size={24} style={{ flexShrink: 0 }} />
                <div>
                  <h4 style={{ fontWeight: '600', marginBottom: '0.25rem' }}>Missing indexes on large tables</h4>
                  <p style={{ fontSize: '0.85rem', color: 'var(--text-secondary)' }}>Detected 3 tables over 1GB with high sequential scan counts. Adding recommended indexes could save ~640 CPU-sec/hour.</p>
                </div>
                <button style={{ marginLeft: 'auto', background: 'none', border: 'none', color: 'white', cursor: 'pointer' }}><ChevronRight /></button>
              </div>
              <div style={{ display: 'flex', gap: '1rem', alignItems: 'flex-start', padding: '1rem', borderRadius: '12px', background: 'rgba(245, 158, 11, 0.1)', borderLeft: '4px solid var(--severity-warning)' }}>
                <Activity color="var(--severity-warning)" size={24} style={{ flexShrink: 0 }} />
                <div>
                  <h4 style={{ fontWeight: '600', marginBottom: '0.25rem' }}>shared_buffers is undersized</h4>
                  <p style={{ fontSize: '0.85rem', color: 'var(--text-secondary)' }}>Current value is 128MB on a 16GB system. Recommendation is 4GB. This is causing a low cache hit ratio (73%).</p>
                </div>
                <button style={{ marginLeft: 'auto', background: 'none', border: 'none', color: 'white', cursor: 'pointer' }}><ChevronRight /></button>
              </div>
            </div>
          </div>
        </section>

        <div className="category-grid">
          {categories.map((cat, i) => (
            <div key={cat.id} className="glass-card category-card animate-fade-in" style={{ animationDelay: `${0.2 + i * 0.05}s` }}>
              <div className="category-header">
                <div style={{ display: 'flex', alignItems: 'center', gap: '0.75rem' }}>
                  {cat.id === 'config' && <Settings size={20} color="var(--accent-blue)" />}
                  {cat.id === 'indexes' && <Zap size={20} color="var(--severity-critical)" />}
                  {cat.id === 'queries' && <Search size={20} color="var(--severity-warning)" />}
                  {cat.id === 'vacuum' && <Shield size={20} color="var(--severity-ok)" />}
                  {cat.id === 'conn' && <CheckCircle2 size={20} color="var(--severity-ok)" />}
                  {cat.id === 'cache' && <Zap size={20} color="var(--severity-critical)" />}
                  <span className="category-title">{cat.name}</span>
                </div>
                <span className={`badge badge-${cat.severity}`}>
                  {cat.score}%
                </span>
              </div>
              <div style={{ display: 'flex', justifyContent: 'space-between', alignItems: 'flex-end', marginTop: '1.5rem' }}>
                <span className="finding-count">{cat.findings} findings</span>
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
