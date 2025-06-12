package cache

import (
	"fmt"
	"os"

	"github.com/jasondellaluce/synchro/pkg/cache"
	"github.com/sirupsen/logrus"
	"github.com/spf13/cobra"
)

func init() {
	CacheCmd.AddCommand(clearCmd)
}

var CacheCmd = &cobra.Command{
	Use:   "cache",
	Short: "Manages the cache",
}

var clearCmd = &cobra.Command{
	Use:   "clear",
	Short: "Clears the cache",
	RunE: func(cmd *cobra.Command, args []string) error {
		cachePath, err := cache.GetCachePath()
		if err != nil {
			return err
		}

		if _, err := os.Stat(cachePath); os.IsNotExist(err) {
			logrus.Infof("Cache file not found, nothing to do.")
			return nil
		}

		if err := os.Remove(cachePath); err != nil {
			return fmt.Errorf("failed to remove cache file: %w", err)
		}

		logrus.Infof("Cache cleared successfully from %s", cachePath)
		return nil
	},
}
