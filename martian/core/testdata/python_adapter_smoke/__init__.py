"""Python adapter smoke stage for core runtime tests."""

import martian


def main(args, outs):
    martian.log_info("python adapter smoke log")
    martian.update_progress("python adapter smoke progress")
    with open(outs.result, "w") as result_file:
        result_file.write(args.message + "\n")
