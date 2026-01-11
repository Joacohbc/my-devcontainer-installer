#!/usr/bin/env python3
import subprocess
import json
import os
import sys
import re
from pathlib import Path

# Colors
GREEN = "\033[92m"
YELLOW = "\033[93m"
RED = "\033[91m"
RESET = "\033[0m"
BOLD = "\033[1m"

def print_success(msg):
    print(f"{GREEN}{msg}{RESET}")

def print_warning(msg):
    print(f"{YELLOW}{msg}{RESET}")

def print_error(msg):
    print(f"{RED}{msg}{RESET}")

def print_info(msg):
    print(f"{BOLD}{msg}{RESET}")

def run_command(command, capture_output=True, text=True):
    """Run a shell command."""
    try:
        result = subprocess.run(
            command, 
            shell=True, 
            check=True, 
            capture_output=capture_output, 
            text=text
        )
        if result.stdout:
            return result.stdout.strip()
        return None
    except subprocess.CalledProcessError as e:
        if not capture_output:
            # If we were not capturing output, the error is likely already printed
            return None
        print_error(f"Error running command: {command}")
        print(e.stderr)
        sys.exit(1)

def get_container_ip():
    """Get IP address of devcontainer-ssh container."""
    try:
        inspect_output = run_command("docker inspect devcontainer-ssh")
        data = json.loads(inspect_output)
        networks = data[0]["NetworkSettings"]["Networks"]
        
        network_names = list(networks.keys())
        
        if len(network_names) == 1:
            return networks[network_names[0]]["IPAddress"]
        elif len(network_names) > 1:
            print_warning("Multiple networks detected. Please choose the correct IP address from the following:")
            ips = []
            for i, name in enumerate(network_names):
                ip = networks[name]["IPAddress"]
                ips.append(ip)
                print(f"{i}) {ip} ({name})")
            
            while True:
                try:
                    selection = int(input("Enter the number corresponding to the desired IP address: "))
                    if 0 <= selection < len(ips):
                        return ips[selection]
                    print_error("Invalid selection.")
                except ValueError:
                    print_error("Please enter a number.")
        else:
            print_error("No networks found for devcontainer-ssh.")
            sys.exit(1)
            
    except Exception as e:
        print_error(f"Error getting container IP: {e}")
        sys.exit(1)

def get_devuser_password():
    """Extract devuser password from docker logs."""
    try:
        # Running docker compose logs and filtering with python
        # Equivalent to: docker compose logs devcontainer-ssh | grep "devuser password:" | awk '{print $NF}'
        
        # Note: 'docker compose logs' might output to stderr sometimes, so we capture everything
        result = subprocess.run(
            "docker compose logs devcontainer-ssh", 
            shell=True,
            capture_output=True,
            text=True
        )
        
        output = result.stdout + result.stderr
        ansi_escape = re.compile(r'\x1B(?:[@-Z\\-_]|\[[0-?]*[ -/]*[@-~])')
        
        last_password = None
        for line in output.splitlines():
            clean_line = ansi_escape.sub('', line)
            if "devuser password:" in clean_line:
                # Store the found password but keep searching for potentially newer ones
                parts = clean_line.split()
                if parts:
                    last_password = parts[-1]
        
        if last_password:
            return last_password
                
        return "Not found"
    except Exception as e:
        print_error(f"Error getting password: {e}")
        return "Error"

def generate_ssh_key(key_path):
    """Generate SSH key if it doesn't exist."""
    key_file = Path(key_path).expanduser()
    if not key_file.exists():
        print_info(f"\nSetting up SSH key authentication...")
        print(f"Generating key at {key_file}...")
        # -N "" for no passphrase, -q for quiet
        run_command(f'ssh-keygen -t ed25519 -f "{key_file}" -N "" -q', capture_output=False)
    else:
        print_info(f"\nSSH key already exists at {key_file}. Skipping generation.")

def update_ssh_config(ip_address, key_path):
    """Update ~/.ssh/config with the new host entry."""
    config_path = Path("~/.ssh/config").expanduser()
    key_path_expanded = Path(key_path).expanduser()
    
    host_entry = f"""
Host devcontainer-ssh
    HostName {ip_address}
    User devuser
    IdentityFile {key_path_expanded}
"""
    
    # Create file if it doesn't exist
    if not config_path.exists():
        config_path.parent.mkdir(mode=0o700, parents=True, exist_ok=True)
        config_path.touch(mode=0o600)

    current_config = config_path.read_text()
    
    if "Host devcontainer-ssh" in current_config:
        print_info("\nEntry for 'devcontainer-ssh' already exists in ~/.ssh/config. Updating it...")
        # Simple parsing to replace the block could be complex. 
        # For simplicity, we might inform the user or try a regex replacement if strictly needed.
        # But a robust way is to read lines, remove the old block, and append new.
        
        new_lines = []
        skip = False
        for line in current_config.splitlines():
            if line.strip().startswith("Host devcontainer-ssh"):
                skip = True
            elif skip and line.strip().startswith("Host "):
                skip = False
                new_lines.append(line)
            elif not skip:
                new_lines.append(line)
                
        # Append new block
        new_lines.append(host_entry)
        config_path.write_text("\n".join(new_lines) + "\n")
        
    else:
        print_info("\nAdding 'devcontainer-ssh' to ~/.ssh/config...")
        with open(config_path, "a") as f:
            f.write(host_entry)

def main():
    final_ip = get_container_ip()
    password = get_devuser_password()
    
    print_info(f"\nThe IP address of the devcontainer-ssh is: {final_ip}")
    print_info(f"The password for devuser is below (selecting it usually copies it):")
    print_success(f"\n    {password}\n")
    print_warning(f"PLEASE COPY THE PASSWORD ABOVE!")
    print_warning(f"You will need to PASTE it when prompted in the next step.")
    
    key_path = "~/.ssh/id_devcontainer"
    generate_ssh_key(key_path)
    
    print_info("\nCopying SSH key to devcontainer-ssh...")
    # Use interactive call for ssh-copy-id so user can enter password
    try:
        subprocess.run(
            f"ssh-copy-id -o StrictHostKeyChecking=no -i {key_path}.pub devuser@{final_ip}", 
            shell=True, 
            check=True
        )
    except subprocess.CalledProcessError:
        print_error("Failed to copy SSH key. Please check your password and try again.")
        sys.exit(1)

    update_ssh_config(final_ip, key_path)
    
    print_success("\nSuccess! You can now log in using:")
    print_info("  ssh devcontainer-ssh")
    print_info("\nOr, attempting to log in now...")
    
    # Login
    subprocess.run(f"ssh devcontainer-ssh", shell=True)

if __name__ == "__main__":
    main()
