import { Card, CardContent } from '@/components/ui/Card';
import { JobStatusBadge } from './JobStatusBadge';
import { formatDate } from '@/lib/utils';
import type { Job } from '@/types';

interface JobCardProps {
  job: Job;
}

export function JobCard({ job }: JobCardProps) {
  return (
    <Card>
      <CardContent className="p-4">
        <div className="flex items-start justify-between mb-2">
          <div>
            <p className="text-sm font-mono text-gray-600">ID: {job.id}</p>
            <p className="font-medium">Type: <code className="text-sm bg-gray-100 px-1 py-0.5 rounded">{job.type}</code></p>
          </div>
          <JobStatusBadge status={job.status} />
        </div>
        
        <div className="space-y-1 text-sm">
          <p>Attempts: {job.attempts}</p>
          {job.error_message?.Valid && (
            <p className="text-red-600">Error: {job.error_message.String}</p>
          )}
          <p className="text-gray-500">Created: {formatDate(job.created_at)}</p>
          <p className="text-gray-500">Updated: {formatDate(job.updated_at)}</p>
        </div>
      </CardContent>
    </Card>
  );
}