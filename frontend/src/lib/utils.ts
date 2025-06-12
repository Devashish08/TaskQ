import { clsx, type ClassValue } from 'clsx';
import { twMerge } from 'tailwind-merge';

export function cn(...inputs: ClassValue[]) {
  return twMerge(clsx(inputs));
}

export function formatDate(date: string): string {
  return new Date(date).toLocaleString();
}

export const jobTemplates = {
  log_payload: {
    message: "Hello from TaskQ React Frontend!"
  },
  failing_job: {
    message: "This job will be retried",
    force_fail: false
  },
  long_job: {
    task_id: 12345
  },
};