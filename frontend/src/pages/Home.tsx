import { Card, CardContent, CardDescription, CardHeader, CardTitle } from '@/components/ui/Card';
import { CheckCircle, Clock, RefreshCw, Server } from 'lucide-react';

export function Home() {
  const features = [
    {
      icon: <Server className="h-8 w-8 text-primary-600" />,
      title: 'RESTful API',
      description: 'Simple HTTP endpoints for job submission and status tracking',
    },
    {
      icon: <RefreshCw className="h-8 w-8 text-primary-600" />,
      title: 'Retry Mechanism',
      description: 'Automatic retries for failed jobs with configurable attempts',
    },
    {
      icon: <Clock className="h-8 w-8 text-primary-600" />,
      title: 'Async Processing',
      description: 'Background workers process jobs without blocking your application',
    },
    {
      icon: <CheckCircle className="h-8 w-8 text-primary-600" />,
      title: 'Reliable Storage',
      description: 'PostgreSQL for persistence and Redis for fast queue operations',
    },
  ];

  return (
    <div className="space-y-8">
      <div className="text-center">
        <h1 className="text-4xl font-bold tracking-tight">Welcome to TaskQ</h1>
        <p className="mt-4 text-xl text-gray-600">
          A robust job queue system built with Go, PostgreSQL, and Redis
        </p>
      </div>

      <Card>
        <CardHeader>
          <CardTitle>About TaskQ</CardTitle>
        </CardHeader>
        <CardContent>
          <p className="text-gray-600">
            TaskQ is a backend service designed to accept tasks via an API, queue them reliably, 
            and process them asynchronously using background workers. This allows primary application 
            services to offload long-running or deferrable tasks, improving responsiveness and system resilience.
          </p>
        </CardContent>
      </Card>

      <div className="grid grid-cols-1 md:grid-cols-2 gap-6">
        {features.map((feature, index) => (
          <Card key={index}>
            <CardHeader>
              <div className="flex items-center space-x-4">
                {feature.icon}
                <CardTitle className="text-lg">{feature.title}</CardTitle>
              </div>
            </CardHeader>
            <CardContent>
              <CardDescription>{feature.description}</CardDescription>
            </CardContent>
          </Card>
        ))}
      </div>
    </div>
  );
}