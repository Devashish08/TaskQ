import { JobSubmissionForm } from '@/components/jobs/JobSubmissionForm';
import { JobList } from '@/components/jobs/JobList';

export function Jobs() {
  return (
    <div className="grid grid-cols-1 lg:grid-cols-2 gap-8">
      <div>
        <JobSubmissionForm />
      </div>
      <div>
        <JobList />
      </div>
    </div>
  );
}