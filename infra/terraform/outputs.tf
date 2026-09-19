output "cluster_name" {
  description = "EKS cluster name for a separately authorized bootstrap process."
  value       = aws_eks_cluster.this.name
}

output "cluster_endpoint" {
  description = "Private EKS API endpoint. Treat it as operational metadata and expose it only to authorized operators."
  value       = aws_eks_cluster.this.endpoint
  sensitive   = true
}

output "cluster_certificate_authority_data" {
  description = "Base64 CA data for an authorized bootstrap client."
  value       = aws_eks_cluster.this.certificate_authority[0].data
  sensitive   = true
}

output "vpc_id" {
  description = "Reference VPC identifier."
  value       = aws_vpc.this.id
}

output "private_subnet_ids" {
  description = "Private subnet identifiers used by EKS."
  value       = [for subnet in aws_subnet.private : subnet.id]
}

output "oidc_issuer" {
  description = "OIDC issuer endpoint. No provider or workload IAM role is created by this reference."
  value       = aws_eks_cluster.this.identity[0].oidc[0].issuer
}
