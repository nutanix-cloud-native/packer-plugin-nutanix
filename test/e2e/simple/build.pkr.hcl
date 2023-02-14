build {
  source "nutanix.centos" {
    name = "centos"
  }

  provisioner "shell" {
    only = ["nutanix.centos"]
    environment_vars = [
      "FOO=hello world",
    ]
    inline = [
      "echo \"FOO is $FOO\" > example.txt",
    ]
  }
}
