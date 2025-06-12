import axios from 'axios';
import type { Job, JobSubmission } from '@/types';

const API_BASE_URL = import.meta.env.VITE_API_URL || '/api/v1';

const api = axios.create({
  baseURL: API_BASE_URL,
  headers: {
    'Content-Type': 'application/json',
  },
});

export const jobsApi = {
  async submitJob(data: JobSubmission): Promise<{ id: string; status: string }> {
    const response = await api.post('/jobs', data);
    return response.data;
  },

  async getJobs(): Promise<Job[]> {
    const response = await api.get('/jobs');
    return response.data;
  },

  async getJob(id: string): Promise<Job> {
    const response = await api.get(`/jobs/${id}`);
    return response.data;
  },
};