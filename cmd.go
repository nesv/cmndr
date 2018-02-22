package cmndr

import (
	"flag"
	"fmt"
	"os"
	"sort"
	"text/tabwriter"

	"github.com/pkg/errors"
)

// RunFunc defines the arity and return signatures of a function that a Cmd
// will run.
type RunFunc func(cmd *Cmd, args []string) error

// Cmd defines the structure of a command that can be run.
type Cmd struct {
	// The name of the command.
	Name string

	// A brief, single line description of the command.
	Description string

	// A *flag.FlagSet for registering command-line flags for the command.
	Flags *flag.FlagSet

	// Will be nil unless subcommands are registered with the AddCmd()
	// method.
	Commands map[string]*Cmd

	// The function to run.
	Run RunFunc
}

func newUsage(c *Cmd) func() {
	return func() {
		if c.Flags == nil {
			c.Flags = flag.NewFlagSet(c.Name, flag.ExitOnError)
		}
		fmt.Fprintf(os.Stderr, "%s - %s\n", c.Name, c.Description)
		printSubcommands(c)
		fmt.Fprintln(os.Stderr, "\nFlags")
		c.Flags.PrintDefaults()
	}
}

// newHelpCmd is called by New() to add a "help" subcommand to parent.
func newHelpCmd(parent *Cmd) *Cmd {
	descr := fmt.Sprintf("Print the help message for %s or a subcommand", parent.Name)
	return &Cmd{
		Name:        "help",
		Description: descr,
		Run: func(cmd *Cmd, args []string) error {
			// The command to print the help message for.
			var pp *Cmd

			if len(args) == 0 || parent.Commands == nil {
				// In this situation, the user has just run
				//
				//	$ cmd help
				//
				// or, they have run
				//
				//	$ cmd help foo
				//
				// but "cmd" has no registered subcommands.
				pp = parent
			} else if sub, ok := parent.Commands[args[0]]; ok {
				// The user has run
				//
				//	$ cmd help foo
				//
				// and the "foo" subcommand has been
				// registered.
				pp = sub
			}

			// If pp hasn't been set, this means that the user has
			// intended to print the help message of a subcommand,
			// but that subcommand does not exist.
			if pp == nil {
				return errors.Errorf("no such command: %q", args[0])
			}

			if pp.Flags == nil {
				fmt.Fprintf(os.Stderr, "%s - %s\n", pp.Name, pp.Description)
				printSubcommands(pp)
			} else {
				pp.Flags.Usage()
			}
			return nil
		},
	}
}

// printSubcommands is a helper function, used when calling a "help"
// subcommand; it prints all of the registered subcommands of c, if any.
func printSubcommands(c *Cmd) {
	if c.Commands == nil {
		return
	}

	fmt.Fprintln(os.Stderr, "\nCommands")

	// Gather a list of all subcommand names, and sort them (for
	// consistent output).
	var subNames []string
	for name, _ := range c.Commands {
		subNames = append(subNames, name)
	}
	sort.Strings(subNames)

	tw := tabwriter.NewWriter(os.Stderr, 0, 4, 1, ' ', 0)
	defer tw.Flush()
	for _, name := range subNames {
		fmt.Fprintf(tw, "\t\t%s\t%s\n", name, c.Commands[name].Description)
	}
}

// New is a convenience function for creating and returning a new *Cmd.
//
// New will automatically add a "help" subcommand that, when called with no
// arguments, will print the help message for its parent command. If any
// arguments are provided to the "help" subcommand, only the first argument
// will be consulted, and it will print the help message for the specified
// subcommand.
//
// Note, that the following two command-line calls are effectively the same:
//
//	$ my-command help <subcommand>
//	$ my-command <subcommand> help
//
func New(name string, run RunFunc) *Cmd {
	c := &Cmd{
		Name:  name,
		Flags: flag.NewFlagSet(name, flag.ExitOnError),
		Run:   run,
	}
	c.Flags.Usage = newUsage(c)
	c.AddCmd(newHelpCmd(c))
	return c
}

// AddCmd registers a subcommand.
//
// AddCmd will panic if the given cmd's Name field is an empty string.
// If there is a subcommand already registered with the same name, it will be
// replaced.
func (c *Cmd) AddCmd(cmd *Cmd) {
	if c.Commands == nil {
		c.Commands = make(map[string]*Cmd)
	}
	if cmd.Name == "" {
		panic("cannot add nameless subcommand")
	}
	c.Commands[cmd.Name] = cmd
}

// Exec parses the arguments provided on the command line. This is the
// method that should be called from the outer-most command (e.g. the
// "root" command).
//
// It is essentially a short-hand invocation of
//
//	c.ExecArgs(os.Args[1:])
//
func (c *Cmd) Exec() {
	c.ExecArgs(os.Args[1:])
}

// ExecArgs executes c.Run with the given arguments. If c.Run == nil,
// and no subcommand was provided as a positional argument, this method will
// print a usage message, and exit.
//
// To customize the usage message that is printed, set c.Flags.Usage (refer to
// the documentation for flag.FlagSet).
func (c *Cmd) ExecArgs(args []string) {
	// Make sure there is a non-nil flag set.
	if c.Flags == nil {
		c.Flags = flag.NewFlagSet(os.Args[0], flag.ExitOnError)
	}

	// Parse the given arguments.
	if err := c.Flags.Parse(args); err != nil {
		fmt.Fprintln(os.Stderr, "error parsing arguments:", err)
		os.Exit(2)
	}

	// If we have some registered subcommands, and the first positional
	// argument matches the name of one of the registered subcommands,
	// execute it.
	if c.Commands != nil && c.Flags.Arg(0) != "" {
		if sub, ok := c.Commands[c.Flags.Arg(0)]; ok {
			// Our first positional argument refers to a registered
			// subcommand.
			//
			// Run that subcommand.
			if c.Flags.NArg() > 1 {
				sub.ExecArgs(c.Flags.Args()[1:])
			} else {
				sub.ExecArgs(nil)
			}
			return
		}
	}

	// No subcommand was provided, and our main RunFunc is nil. Print a
	// usage message, and exit.
	if c.Run == nil {
		c.Flags.Usage()
		os.Exit(1)
	}

	// Call our RunFunc.
	if err := c.Run(c, c.Flags.Args()); err != nil {
		fmt.Fprintln(os.Stderr, "error:", err)
		os.Exit(1)
	}
}
