document.addEventListener('DOMContentLoaded', () => {
    const submitJobForm = document.getElementById('submitJobForm');
    const jobTypeSelect = document.getElementById('jobType');
    const jobPayloadTextarea = document.getElementById('jobPayload');
    const submissionApiResponsePre = document.getElementById('submissionApiResponse');

    const getJobStatusForm = document.getElementById('getJobStatusForm');
    const queryJobIdInput = document.getElementById('queryJobId');
    const statusApiResponsePre = document.getElementById('statusApiResponse');

    // Base URL for the API
    const API_BASE_URL = 'https://taskq-jct8.onrender.com/api/v1';

    // Handle Job Submission
    if (submitJobForm) {
        submitJobForm.addEventListener('submit', async (event) => {
            event.preventDefault();
            submissionApiResponsePre.textContent = 'Submitting...';

            const jobType = jobTypeSelect.value;
            const payloadString = jobPayloadTextarea.value;
            let payload;

            try {
                payload = payloadString ? JSON.parse(payloadString) : {}; // Allow empty payload as {}
            } catch (e) {
                submissionApiResponsePre.textContent = `Error: Payload is not valid JSON.\n${e.toString()}`;
                return;
            }

            try {
                const response = await fetch(`${API_BASE_URL}/jobs`, {
                    method: 'POST',
                    headers: {
                        'Content-Type': 'application/json',
                    },
                    body: JSON.stringify({
                        type: jobType,
                        payload: payload,
                    }),
                });

                const responseData = await response.json();
                submissionApiResponsePre.textContent = `Status: ${response.status}\n\n${JSON.stringify(responseData, null, 2)}`;
                
                // If submission is successful and returns a job_id, populate the query field
                if (response.ok && responseData.job_id) {
                    queryJobIdInput.value = responseData.job_id;
                } else if(response.ok && responseData.job_type) { // Handle your current API response
                    queryJobIdInput.value = responseData.job_type;
                }

            } catch (error) {
                submissionApiResponsePre.textContent = `Network or other error submitting job:\n${error.toString()}`;
                console.error('Error submitting job:', error);
            }
        });
    }

    // Handle Get Job Status
    if (getJobStatusForm) {
        getJobStatusForm.addEventListener('submit', async (event) => {
            event.preventDefault();
            statusApiResponsePre.textContent = 'Fetching status...';

            const jobId = queryJobIdInput.value.trim();

            if (!jobId) {
                statusApiResponsePre.textContent = 'Error: Job ID cannot be empty.';
                return;
            }

            try {
                const response = await fetch(`${API_BASE_URL}/jobs/${jobId}`, {
                    method: 'GET',
                });

                const responseData = await response.json();
                statusApiResponsePre.textContent = `Status: ${response.status}\n\n${JSON.stringify(responseData, null, 2)}`;

            } catch (error) {
                statusApiResponsePre.textContent = `Network or other error fetching status:\n${error.toString()}`;
                console.error('Error fetching job status:', error);
            }
        });
    }
});