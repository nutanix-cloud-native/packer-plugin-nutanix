build {
  source "nutanix.centos" {
    name = "centos"
  }

  provisioner "shell" {
    environment_vars = [
      "FOO=hello world",
    ]
    inline = [
      "echo \"FOO is $FOO\" > example.txt",
    ]
  }
}
