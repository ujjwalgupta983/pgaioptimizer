import React, { useState } from 'react';
import { AlertCircle, Activity, Info, CheckCircle2, ChevronDown, ChevronUp, Terminal } from 'lucide-react';
import type { Finding } from '../api/client';

interface FindingsListProps {
  findings: Finding[];
}

export const FindingsList: React.FC<FindingsListProps> = ({ findings }) => {
  if (!findings || findings.length === 0) {
    return <div style={{ color: 'var(--text-secondary)' }}>No issues found in this category! 🎉</div>;
  }

  return (
    <div style={{ display: 'flex', flexDirection: 'column', gap: '1rem' }}>
      {findings.map((finding, idx) => (
        <FindingItem key={idx} finding={finding} />
      ))}
    </div>
  );
};

const FindingItem: React.FC<{ finding: Finding }> = ({ finding }) => {
  const [expanded, setExpanded] = useState(false);

  const getIcon = () => {
    switch (finding.severity) {
      case 'critical': return <AlertCircle color="var(--severity-critical)" size={24} />;
      case 'warning': return <Activity color="var(--severity-warning)" size={24} />;
      case 'info': return <Info color="var(--accent-blue)" size={24} />;
      default: return <CheckCircle2 color="var(--severity-ok)" size={24} />;
    }
  };

  const getBgColor = () => {
    switch (finding.severity) {
      case 'critical': return 'rgba(239, 68, 68, 0.1)';
      case 'warning': return 'rgba(245, 158, 11, 0.1)';
      case 'info': return 'rgba(59, 130, 246, 0.1)';
      default: return 'rgba(34, 197, 94, 0.1)';
    }
  };

  const getBorderColor = () => {
    switch (finding.severity) {
      case 'critical': return 'var(--severity-critical)';
      case 'warning': return 'var(--severity-warning)';
      case 'info': return 'var(--accent-blue)';
      default: return 'var(--severity-ok)';
    }
  };

  return (
    <div style={{ 
      display: 'flex', 
      flexDirection: 'column',
      padding: '1rem', 
      borderRadius: '12px', 
      background: getBgColor(), 
      borderLeft: `4px solid ${getBorderColor()}`,
      transition: 'all 0.2s ease-in-out'
    }}>
      <div 
        style={{ display: 'flex', gap: '1rem', alignItems: 'flex-start', cursor: 'pointer' }}
        onClick={() => setExpanded(!expanded)}
      >
        <div style={{ flexShrink: 0, marginTop: '2px' }}>{getIcon()}</div>
        <div style={{ flex: 1 }}>
          <h4 style={{ fontWeight: '600', marginBottom: '0.25rem' }}>{finding.title}</h4>
          <p style={{ fontSize: '0.85rem', color: 'var(--text-secondary)' }}>{finding.description}</p>
        </div>
        <button style={{ background: 'none', border: 'none', color: 'white', cursor: 'pointer', padding: '0.25rem' }}>
          {expanded ? <ChevronUp size={20} /> : <ChevronDown size={20} />}
        </button>
      </div>

      {expanded && (
        <div style={{ marginTop: '1rem', paddingTop: '1rem', borderTop: '1px solid rgba(255,255,255,0.1)', animation: 'fadeIn 0.2s ease-in-out' }}>
          <div style={{ display: 'grid', gridTemplateColumns: '1fr 1fr', gap: '1rem', marginBottom: '1rem' }}>
            <div>
              <span style={{ fontSize: '0.75rem', color: 'var(--text-secondary)', textTransform: 'uppercase' }}>Current Value</span>
              <div style={{ fontWeight: '500' }}>{finding.current_value}</div>
            </div>
            {finding.recommended_value && (
              <div>
                <span style={{ fontSize: '0.75rem', color: 'var(--text-secondary)', textTransform: 'uppercase' }}>Recommended</span>
                <div style={{ fontWeight: '500', color: 'var(--severity-ok)' }}>{finding.recommended_value}</div>
              </div>
            )}
          </div>
          
          <div style={{ marginBottom: '1rem' }}>
            <span style={{ fontSize: '0.75rem', color: 'var(--text-secondary)', textTransform: 'uppercase' }}>Impact</span>
            <div style={{ fontSize: '0.9rem' }}>{finding.impact}</div>
          </div>

          {finding.sql_fix && (
            <div style={{ background: 'rgba(0,0,0,0.5)', padding: '1rem', borderRadius: '8px', border: '1px solid rgba(255,255,255,0.1)' }}>
              <div style={{ display: 'flex', alignItems: 'center', gap: '0.5rem', marginBottom: '0.5rem', color: 'var(--text-secondary)' }}>
                <Terminal size={14} />
                <span style={{ fontSize: '0.75rem', textTransform: 'uppercase' }}>SQL Fix</span>
              </div>
              <pre style={{ margin: 0, whiteSpace: 'pre-wrap', fontFamily: 'monospace', fontSize: '0.85rem', color: 'var(--accent-blue)' }}>
                {finding.sql_fix}
              </pre>
            </div>
          )}
        </div>
      )}
    </div>
  );
};
