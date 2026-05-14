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
    <div className="flex flex-col gap-4">
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

  return (
    <div className={`finding-item ${finding.severity}`}>
      <div 
        className="finding-item-header"
        onClick={() => setExpanded(!expanded)}
      >
        <div style={{ flexShrink: 0, marginTop: '2px' }}>{getIcon()}</div>
        <div style={{ flex: 1 }}>
          <h4 className="font-bold text-primary mb-2">{finding.title}</h4>
          <p className="text-secondary text-sm">{finding.description}</p>
        </div>
        <button style={{ background: 'none', border: 'none', color: 'var(--text-secondary)', cursor: 'pointer', padding: '0.25rem' }}>
          {expanded ? <ChevronUp size={20} /> : <ChevronDown size={20} />}
        </button>
      </div>

      {expanded && (
        <div className="mt-4 pt-4 animate-fade-in" style={{ borderTop: '1px solid var(--card-border)' }}>
          <div className="grid-2 mb-4" style={{ display: 'grid', gridTemplateColumns: '1fr 1fr', gap: '1rem' }}>
            <div>
              <span className="text-xs text-muted">Current Value</span>
              <div className="font-medium mt-2">{finding.current_value}</div>
            </div>
            {finding.recommended_value && (
              <div>
                <span className="text-xs text-muted">Recommended</span>
                <div className="font-medium mt-2 text-ok" style={{ color: 'var(--severity-ok)' }}>{finding.recommended_value}</div>
              </div>
            )}
          </div>
          
          <div className="mb-4">
            <span className="text-xs text-muted">Impact</span>
            <div className="text-sm mt-2">{finding.impact}</div>
          </div>

          {finding.sql_fix && (
            <div className="terminal-block">
              <div className="terminal-header">
                <Terminal size={14} />
                <span className="text-xs">SQL Fix</span>
              </div>
              <pre className="terminal-code">
                {finding.sql_fix}
              </pre>
            </div>
          )}
        </div>
      )}
    </div>
  );
};
