export interface Job {
    id: string;
    type: string;
    payload: Record<string, any>;
    status: 'pending' | 'running' | 'completed' | 'failed';
    attempts: number;
    error_message?: {
      String: string;
      Valid: boolean;
    };
    created_at: string;
    updated_at: string;
    completed_at?: string;
  }
  
  export interface JobSubmission {
    type: string;
    payload: Record<string, any>;
  }
  
  export interface ApiResponse<T> {
    data?: T;
    error?: string;
  }