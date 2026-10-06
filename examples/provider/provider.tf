terraform {
  required_providers {
    victorops = {
      source = "reecetech/victorops"
    }
  }
}

provider "victorops" {
  api_id  = var.victorops_api_id  # or set VO_API_ID
  api_key = var.victorops_api_key # or set VO_API_KEY
}
