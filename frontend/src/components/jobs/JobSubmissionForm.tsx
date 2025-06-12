import { useState } from 'react';
import { useMutation, useQueryClient } from '@tanstack/react-query';
import { Card, CardContent, CardDescription, CardHeader, CardTitle } from '@/components/ui/Card';
import { Button } from '@/components/ui/Button';
import { Select } from '@/components/ui/Select';
import { Textarea } from '@/components/ui/Textarea';
import { jobsApi } from '@/lib/api';
import { jobTemplates } from '@/lib/utils';

export function JobSubmissionForm() {
  const [jobType, setJobType] = useState<keyof typeof jobTemplates>('log_payload');
  const [payload, setPayload] = useState(JSON.stringify(jobTemplates.log_payload, null, 2));
  const [submitResponse, setSubmitResponse] = useState<any>(null);
  const queryClient = useQueryClient();

  const submitJob = useMutation({
    mutationFn: jobsApi.submitJob,
    onSuccess: (data) => {
      setSubmitResponse(data);
      queryClient.invalidateQueries({ queryKey: ['jobs'] });
    },
    onError: (error: any) => {
      setSubmitResponse({ error: error.message });
    },
  });

  const handleSubmit = (e: React.FormEvent) => {
    e.preventDefault();
    try {
      const parsedPayload = JSON.parse(payload);
      submitJob.mutate({ type: jobType, payload: parsedPayload });
    } catch (error) {
      setSubmitResponse({ error: 'Invalid JSON payload' });
    }
  };

  const handleJobTypeChange = (e: React.ChangeEvent<HTMLSelectElement>) => {
    const newType = e.target.value as keyof typeof jobTemplates;
    setJobType(newType);
    setPayload(JSON.stringify(jobTemplates[newType] || {}, null, 2));
  };

  return (
    <Card>
      <CardHeader>
        <CardTitle>Submit New Job</CardTitle>
        <CardDescription>
          Select a job type and provide a JSON payload to enqueue a new task
        </CardDescription>
      </CardHeader>
      <CardContent>
        <form onSubmit={handleSubmit} className="space-y-4">
          <div>
            <label htmlFor="jobType" className="block text-sm font-medium mb-2">
              Job Type
            </label>
            <Select id="jobType" value={jobType} onChange={handleJobTypeChange}>
              <option value="log_payload">Log Payload (Success)</option>
              <option value="failing_job">Failing Job (Demonstrates Retries)</option>
              <option value="long_job">Long Running Job (3 seconds)</option>
            </Select>
          </div>

          <div>
            <label htmlFor="payload" className="block text-sm font-medium mb-2">
              Payload (JSON)
            </label>
            <Textarea
              id="payload"
              value={payload}
              onChange={(e) => setPayload(e.target.value)}
              rows={8}
              className="font-mono text-sm"
            />
          </div>

          <Button type="submit" disabled={submitJob.isPending} className="w-full">
            {submitJob.isPending ? 'Submitting...' : 'Submit Job'}
          </Button>
        </form>

        {submitResponse && (
          <div className={`mt-4 p-4 rounded-lg ${submitResponse.error ? 'bg-red-50' : 'bg-green-50'}`}>
            <h4 className={`font-semibold ${submitResponse.error ? 'text-red-800' : 'text-green-800'}`}>
              {submitResponse.error ? 'Error' : 'Success'}
            </h4>
            <pre className="mt-2 text-sm overflow-auto">
              {JSON.stringify(submitResponse, null, 2)}
            </pre>
          </div>
        )}
      </CardContent>
    </Card>
  );
}