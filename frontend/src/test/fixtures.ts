import type { JobCard, JobDetail, JobsPage } from '../api/client'

export const projectManager: JobCard = {
  city: 'London',
  company: 'Global Enterprises',
  country: 'GB',
  currency_inferred: true,
  id: '0e1cba53-25c9-4a39-b39b-4a4755ef158d',
  posted_on: '2023-10-12',
  remote: true,
  salary_display: '80000.00 GBP / year',
  salary_usd_annual: 105600,
  title: 'Project Manager',
}

export const customerSuccessManager: JobCard = {
  city: 'Toronto',
  company: 'ClientBridge',
  country: 'CA',
  currency_inferred: false,
  id: 'da3de5fb-7c7a-406b-9286-9c843cbf5eb7',
  posted_on: '2023-10-23',
  remote: false,
  salary_display: '110000.00 USD / year',
  salary_usd_annual: 110000,
  title: 'Customer Success Manager',
}

export const growthMarketingManager: JobCard = {
  city: 'San Francisco',
  company: 'ScaleRocket',
  country: 'US',
  currency_inferred: true,
  id: '573eca64-7e5d-4a58-aa44-0acdeed5fdce',
  remote: true,
  salary_display: '125000.00 USD / year',
  salary_usd_annual: 125000,
  title: 'Growth Marketing Manager',
}

export const listPage: JobsPage = {
  facets: {
    country: [
      { count: 7, value: 'US' },
      { count: 2, value: 'CA' },
      { count: 2, value: 'GB' },
    ],
  },
  items: [projectManager, customerSuccessManager, growthMarketingManager],
  page: 1,
  per_page: 20,
  total: 3,
}

export const engPage: JobsPage = {
  facets: {
    country: [
      { count: 3, value: 'US' },
      { count: 1, value: 'CA' },
    ],
  },
  items: [
    {
      city: 'Chicago',
      company: 'QualityLoop',
      country: 'US',
      currency_inferred: false,
      id: '5234c9c6-2af3-480f-918d-119283a4ee5c',
      posted_on: '2023-10-15',
      remote: true,
      salary_display: '110000.00 USD / year',
      salary_usd_annual: 110000,
      title: 'QA Automation Engineer',
      title_highlight: [
        { match: false, text: 'QA Automation ' },
        { match: true, text: 'Engineer' },
      ],
    },
    {
      city: 'New York',
      company: 'Tech Innovators Inc.',
      country: 'US',
      currency_inferred: true,
      id: '684f01dc-9b15-4cf6-a570-00623cd7564f',
      posted_on: '2023-10-01',
      remote: false,
      salary_display: '150000.00 USD / year',
      salary_usd_annual: 150000,
      title: 'Senior Software Engineer',
      title_highlight: [
        { match: false, text: 'Senior Software ' },
        { match: true, text: 'Engineer' },
      ],
    },
  ],
  page: 1,
  per_page: 20,
  total: 4,
}

export const cybersecuritySpecialist: JobDetail = {
  city: 'Washington',
  company: 'SecurePath',
  company_type: 'direct_employer',
  country: 'US',
  currency_inferred: false,
  description: 'Monitor security events.',
  id: 'd4d356de-0fa1-4332-8bde-bb240310f19b',
  posted_on: '2023-10-20',
  region: 'DC',
  remote: false,
  salary_display: '135000.00 USD / year',
  salary_usd_annual: 135000,
  title: 'Cybersecurity Specialist',
}

export const jobDetails: Record<string, JobDetail> = {
  [projectManager.id]: {
    ...projectManager,
    company_type: 'direct_employer',
    description: 'Lead projects across various domains.',
  },
  [customerSuccessManager.id]: {
    ...customerSuccessManager,
    company_type: 'direct_employer',
    description: 'Help customers succeed with our platform.',
    region: 'ON',
  },
  [growthMarketingManager.id]: {
    ...growthMarketingManager,
    company_type: 'consulting_agency',
    description: 'Drive user acquisition.',
    region: 'CA',
  },
  [cybersecuritySpecialist.id]: cybersecuritySpecialist,
}
