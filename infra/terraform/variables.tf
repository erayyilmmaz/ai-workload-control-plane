variable "aws_region" {
  description = "AWS region containing the reference EKS control plane."
  type        = string

  validation {
    condition     = can(regex("^[a-z]{2}-[a-z]+-[0-9]+$", var.aws_region))
    error_message = "aws_region must look like us-east-1."
  }
}

variable "cluster_name" {
  description = "Stable, globally unique EKS cluster name."
  type        = string

  validation {
    condition     = can(regex("^[A-Za-z0-9][A-Za-z0-9_-]{0,99}$", var.cluster_name))
    error_message = "cluster_name must be 1-100 characters using letters, numbers, hyphens or underscores."
  }
}

variable "kubernetes_version" {
  description = "EKS Kubernetes version; select a version supported by the chosen AWS region at apply time."
  type        = string
  default     = "1.36"

  validation {
    condition     = can(regex("^[0-9]+\\.[0-9]+$", var.kubernetes_version))
    error_message = "kubernetes_version must be a major.minor value such as 1.36."
  }
}

variable "availability_zones" {
  description = "Two or more availability zones used for private EKS control-plane subnets."
  type        = list(string)

  validation {
    condition     = length(var.availability_zones) >= 2 && length(distinct(var.availability_zones)) == length(var.availability_zones)
    error_message = "At least two distinct availability zones are required."
  }
}

variable "vpc_cidr" {
  description = "IPv4 CIDR for the reference VPC."
  type        = string

  validation {
    condition     = can(cidrnetmask(var.vpc_cidr)) && length(split(".", var.vpc_cidr)) == 4
    error_message = "vpc_cidr must be a valid IPv4 CIDR."
  }
}

variable "private_subnet_cidrs" {
  description = "One private IPv4 CIDR per availability zone. EKS uses these subnets."
  type        = list(string)

  validation {
    condition     = length(var.private_subnet_cidrs) == length(var.availability_zones) && alltrue([for cidr in var.private_subnet_cidrs : can(cidrnetmask(cidr)) && length(split(".", cidr)) == 4])
    error_message = "private_subnet_cidrs must contain one valid CIDR for every availability zone."
  }
}

variable "public_subnet_cidrs" {
  description = "One public IPv4 CIDR per availability zone. These are retained for explicitly approved ingress/NAT design."
  type        = list(string)

  validation {
    condition     = length(var.public_subnet_cidrs) == length(var.availability_zones) && alltrue([for cidr in var.public_subnet_cidrs : can(cidrnetmask(cidr)) && length(split(".", cidr)) == 4])
    error_message = "public_subnet_cidrs must contain one valid CIDR for every availability zone."
  }
}

variable "enable_nat_gateway" {
  description = "Create one NAT gateway for private-subnet egress. It costs money and is disabled in the reference by default."
  type        = bool
  default     = false
}

variable "tags" {
  description = "Additional non-secret tags merged with mandatory AWCP tags."
  type        = map(string)
  default     = {}
}
