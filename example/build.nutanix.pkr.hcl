build {
  sources = [
    "source.nutanix.centos"
  ]
  
  source "nutanix.ubuntu" {
    name = "ubuntu"
  }

  source "nutanix.centos-kickstart" {
    name = "centos-kickstart"
  }

  source "nutanix.ubuntu-autoinstall" {
    name = "ubuntu-autoinstall"
  }

  source "nutanix.windows" {
    name = "windows"
  }

  provisioner "shell" {
    only = ["nutanix.centos", "nutanix.centos-kickstart" ,"nutanix.ubuntu"]
    environment_vars = [
      "FOO=hello world",
    ]
    inline = [
      "echo \"FOO is $FOO\" > example.txt",
    ]
  }

  provisioner "powershell" {
    only = ["nutanix.windows"]
    scripts = ["scripts/win-update.ps1"]
    pause_before = "2m"
  }
  provisioner "windows-restart" {
    only = ["nutanix.windows"]
    restart_timeout = "30m"
  }
}
