import { cn } from '@/lib/utils';

interface JobStatusBadgeProps {
  status: 'pending' | 'running' | 'completed' | 'failed';
}

export function JobStatusBadge({ status }: JobStatusBadgeProps) {
  const statusStyles = {
    pending: 'bg-orange-100 text-orange-800',
    running: 'bg-blue-100 text-blue-800',
    completed: 'bg-green-100 text-green-800',
    failed: 'bg-red-100 text-red-800',
  };

  return (
    <span
      className={cn(
        'inline-flex items-center px-2.5 py-0.5 rounded-full text-xs font-medium uppercase',
        statusStyles[status]
      )}
    >
      {status}
    </span>
  );
}