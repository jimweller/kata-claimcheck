# REQUIRED PARAMETERS
# These variables must be passed
# ---------------------------------------------------------------------------------------------------------------------

variable "queue_name" {
  type        = string
  description = "The name of the queue to create"
}

# OPTIONAL PARAMETERS
# These variables have default values that can be optionally modified.
# ---------------------------------------------------------------------------------------------------------------------

variable "max_receive_count" {
  type        = number
  description = "The number of retries a message has before it's dropped in the DLQ."
  default     = 5
}

variable "visibility_timeout_seconds" {
  type        = number
  description = "The visibility timeout for the queue. An integer from 0 to 43200 (12 hours)."
  default     = 30
}

variable "tags" {
  type        = map(string)
  description = "A map of tags to apply to the S3 Bucket. The key is the tag name and the value is the tag value."
  default     = {}
}

variable "message_retention_seconds" {
  type        = number
  description = "The number of seconds Amazon SQS retains a message for the main queue. Integer representing seconds, from 60 (1 minute) to 1209600 (14 days)."
  default     = 345600
}
