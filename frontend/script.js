document.addEventListener('DOMContentLoaded', () => {
    // --- CONFIG ---
    // Make sure this points to your DEPLOYED backend URL
    const API_BASE_URL = 'https://taskq-jct8.onrender.com/api/v1';

    // --- ELEMENT SELECTORS ---
    const submitJobForm = document.getElementById('submitJobForm');
    const jobTypeSelect = document.getElementById('jobType');
    const jobPayloadTextarea = document.getElementById('jobPayload');
    const submissionApiResponsePre = document.getElementById('submissionApiResponse');
    const submitBtn = document.getElementById('submitBtn');
    const recentJobsListDiv = document.getElementById('recentJobsList');

    // --- INITIAL STATE & HELPERS ---

    // Example payloads for each job type
    const examplePayloads = {
        log_payload: '{\n  "message": "Hello from the live demo!"\n}',
        failing_job: '{\n  "message": "This job will be retried",\n  "force_fail": false\n}',
        long_job: '{\n  "task_id": 12345\n}',
    };

    // Function to set the payload textarea based on selected job type
    const updatePayloadExample = () => {
        jobPayloadTextarea.value = examplePayloads[jobTypeSelect.value] || '';
    };

    // --- RECENT JOBS LOGIC ---

    // Function to render a single job as an HTML card
    const renderJobCard = (job) => {
        const errorHtml = (job.error_message && job.error_message.Valid)
            ? `<p><strong>Error:</strong> <samp>${job.error_message.String}</samp></p>`
            : '';

        const card = document.createElement('article');
        card.className = 'job-card';
        card.innerHTML = `
            <h5>Job ID: ${job.id}</h5>
            <p><strong>Type:</strong> <code>${job.type}</code></p>
            <p>
                <strong>Status:</strong> 
                <span class="status status-${job.status}">${job.status}</span>
                (Attempts: ${job.attempts})
            </p>
            ${errorHtml}
            <small>Last Updated: ${new Date(job.updated_at).toLocaleString()}</small>
        `;
        return card;
    };

    // Function to fetch and display the list of recent jobs
    const fetchAndDisplayRecentJobs = async () => {
        try {
            recentJobsListDiv.setAttribute('aria-busy', 'true');
            const response = await fetch(`${API_BASE_URL}/jobs`);
            if (!response.ok) {
                throw new Error(`API returned status ${response.status}`);
            }
            const jobs = await response.json();

            recentJobsListDiv.innerHTML = ''; // Clear previous content
            if (jobs && jobs.length > 0) {
                jobs.forEach(job => {
                    const jobCard = renderJobCard(job);
                    recentJobsListDiv.appendChild(jobCard);
                });
            } else {
                recentJobsListDiv.innerHTML = '<p>No recent jobs found. Submit one to get started!</p>';
            }
        } catch (error) {
            console.error('Error fetching recent jobs:', error);
            recentJobsListDiv.innerHTML = `<p style="color: red;">Error loading recent jobs: ${error.message}</p>`;
        } finally {
            recentJobsListDiv.setAttribute('aria-busy', 'false');
        }
    };

    // --- JOB SUBMISSION LOGIC ---

    if (submitJobForm) {
        submitJobForm.addEventListener('submit', async (event) => {
            event.preventDefault();
            submissionApiResponsePre.textContent = 'Submitting...';
            submitBtn.setAttribute('aria-busy', 'true');
            submitBtn.disabled = true;

            const jobType = jobTypeSelect.value;
            const payloadString = jobPayloadTextarea.value;
            let payload;

            try {
                payload = payloadString ? JSON.parse(payloadString) : {};
            } catch (e) {
                submissionApiResponsePre.textContent = `Error: Payload is not valid JSON.\n${e.toString()}`;
                submitBtn.setAttribute('aria-busy', 'false');
                submitBtn.disabled = false;
                return;
            }

            try {
                const response = await fetch(`${API_BASE_URL}/jobs`, {
                    method: 'POST',
                    headers: { 'Content-Type': 'application/json' },
                    body: JSON.stringify({ type: jobType, payload: payload }),
                });

                const responseData = await response.json();
                submissionApiResponsePre.textContent = `Status: ${response.status}\n\n${JSON.stringify(responseData, null, 2)}`;
                
                // Trigger an immediate refresh of the recent jobs list
                if (response.ok) {
                    setTimeout(fetchAndDisplayRecentJobs, 500); // Small delay to allow job to be processed
                }

            } catch (error) {
                submissionApiResponsePre.textContent = `Network or other error submitting job:\n${error.toString()}`;
                console.error('Error submitting job:', error);
            } finally {
                submitBtn.setAttribute('aria-busy', 'false');
                submitBtn.disabled = false;
            }
        });
    }

    // --- EVENT LISTENERS & INITIALIZATION ---
    jobTypeSelect.addEventListener('change', updatePayloadExample);

    // Initial setup
    updatePayloadExample(); // Set initial payload example
    fetchAndDisplayRecentJobs(); // Fetch jobs on page load
    setInterval(fetchAndDisplayRecentJobs, 5000); // Refresh every 5 seconds
});