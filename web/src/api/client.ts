import axios from 'axios';

// Define the interfaces based on our Go models
export interface Finding {
    category: string;
    title: string;
    severity: 'critical' | 'warning' | 'info' | 'ok';
    description: string;
    current_value: string;
    recommended_value?: string;
    impact: string;
    sql_fix?: string;
}

export interface CategoryScore {
    category: string;
    score: number;
    weight: number;
    findings: Finding[];
    description: string;
}

export interface ServerInfo {
    version: string;
    version_num: number;
    host: string;
    port: number;
    database: string;
    is_superuser: boolean;
    extensions: string[];
    uptime: string;
    connection_tier: string;
}

export interface HealthReport {
    overall_score: number;
    grade: string;
    categories: CategoryScore[];
    correlations: Finding[];
    server_info: ServerInfo;
    generated_at: string;
    analysis_tier: string;
    duration: number;
}

const apiClient = axios.create({
    baseURL: '/api',
});

export const getLatestReport = async (): Promise<HealthReport> => {
    const response = await apiClient.get<HealthReport>('/report/latest');
    return response.data;
};
